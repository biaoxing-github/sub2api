package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
)

// MonitorScheduler 调度器接口，供 ChannelMonitorService 在 CRUD 时回调，
// 用 setter 注入避免 service ↔ runner 的 wire 依赖环。
type MonitorScheduler interface {
	// Schedule 为指定监控创建（或重置）独立定时任务。
	// 当 m.Enabled=false 时等同于 Unschedule(m.ID)。
	Schedule(m *ChannelMonitor)
	// Unschedule 取消指定监控的定时任务（若存在）。
	Unschedule(id int64)
}

// monitorRunnerSvc 抽出 runner 实际依赖的两个 service 方法：
//   - 启动时加载 enabled monitor
//   - 每次 ticker 触发执行检测
//
// 用接口而非 *ChannelMonitorService 是为了让 runner 单元测试可注入轻量 stub，
// 避免依赖完整的 repo + encryptor 链路。生产实现 *ChannelMonitorService 自然满足。
type monitorRunnerSvc interface {
	ListEnabledMonitors(ctx context.Context) ([]*ChannelMonitor, error)
	RunCheck(ctx context.Context, id int64) ([]*CheckResult, error)
}

// ChannelMonitorRunner 渠道监控调度器。
//
// 设计：
//   - 每个 enabled monitor 对应一个独立 goroutine + ticker（按各自 IntervalSeconds）
//   - Start 时一次性加载所有 enabled monitor 并为每个建立任务
//   - Service 在 Create/Update/Delete 后通过 MonitorScheduler 接口回调，
//     即时重建/取消对应任务（无需轮询 DB）
//   - 实际 HTTP 检测交给 pond 池（容量 monitorWorkerConcurrency），
//     防止突发并发拖垮上游
//
// 历史清理与日聚合维护由 OpsCleanupService 的 cron 触发
// ChannelMonitorService.RunDailyMaintenance（复用 leader lock + heartbeat），
// 不在 runner 职责内。
type ChannelMonitorRunner struct {
	svc            monitorRunnerSvc
	settingService *SettingService

	pool         pond.Pool
	parentCtx    context.Context
	parentCancel context.CancelFunc

	mu      sync.Mutex
	tasks   map[int64]*scheduledMonitor
	wg      sync.WaitGroup
	started bool
	stopped bool
	unsub   func()

	// runtimeKickCh 合并短时间内连续设置更新，避免阻塞管理端保存请求。
	runtimeKickCh chan struct{}

	// inFlight 跟踪正在执行的 monitor.ID。fire 调度前会检查避免重复提交，
	// 防止单次检测耗时 > interval 时同一 monitor 被并发执行。
	inFlight   map[int64]struct{}
	inFlightMu sync.Mutex
}

// scheduledMonitor 单个监控的运行时上下文。
type scheduledMonitor struct {
	id       int64
	name     string
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewChannelMonitorRunner 构造调度器。Start 在 wire 中调用一次。
// settingService 用于在每次 fire 前读取功能开关；传 nil 时视为总是启用（兼容测试）。
//
// pool 在构造时即建好：避免 Start 在 mu 内赋值、fire/Stop 在 mu 外读取的竞态隐患，
// 且 pond.NewPool 创建本身近似零开销，提前建池不会浪费资源。
func NewChannelMonitorRunner(svc *ChannelMonitorService, settingService *SettingService) *ChannelMonitorRunner {
	return newChannelMonitorRunner(svc, settingService)
}

// newChannelMonitorRunner 内部构造，接受最小化接口，便于单元测试注入 stub。
func newChannelMonitorRunner(svc monitorRunnerSvc, settingService *SettingService) *ChannelMonitorRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelMonitorRunner{
		svc:            svc,
		settingService: settingService,
		pool:           pond.NewPool(monitorWorkerConcurrency),
		parentCtx:      ctx,
		parentCancel:   cancel,
		tasks:          make(map[int64]*scheduledMonitor),
		inFlight:       make(map[int64]struct{}),
		runtimeKickCh:  make(chan struct{}, 1),
	}
}

// Start 加载所有 enabled monitor 并为每个建立独立定时任务。
// 调用方需保证只调一次（wire ProvideChannelMonitorRunner 内只调一次）。
func (r *ChannelMonitorRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()

	if r.settingService != nil {
		unsubscribe := r.settingService.SubscribeChannelMonitorRuntime(r.kickRuntimeReconcile)
		r.mu.Lock()
		if r.stopped {
			r.mu.Unlock()
			unsubscribe()
			return
		}
		r.unsub = unsubscribe
		r.wg.Add(1)
		r.mu.Unlock()
		go r.runtimeReconcileLoop()
	}

	if err := r.reconcileRuntime(); err != nil {
		slog.Error("channel_monitor: reconcile runtime failed at startup", "error", err)
	}
	slog.Info("channel_monitor: runner started", "scheduled_tasks", r.taskCount())
}

// kickRuntimeReconcile 非阻塞唤醒运行时对账；连续通知会合并为一次。
func (r *ChannelMonitorRunner) kickRuntimeReconcile() {
	if r == nil {
		return
	}
	select {
	case r.runtimeKickCh <- struct{}{}:
	default:
	}
}

// runtimeReconcileLoop 在设置持久化成功后立即应用 V1/V2 模式变化。
func (r *ChannelMonitorRunner) runtimeReconcileLoop() {
	defer r.wg.Done()
	for {
		select {
		case <-r.parentCtx.Done():
			return
		case <-r.runtimeKickCh:
			if err := r.reconcileRuntime(); err != nil {
				slog.Error("channel_monitor: reconcile runtime after settings update failed", "error", err)
			}
		}
	}
}

// reconcileRuntime 根据当前开关和模式增量对账 V1 定时任务。
// V2 或禁用时取消全部主动探测；V1 时只重建新增或配置变化的任务。
func (r *ChannelMonitorRunner) reconcileRuntime() error {
	if r == nil || r.svc == nil {
		return nil
	}
	if r.settingService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), monitorStartupLoadTimeout)
		runtime := r.settingService.GetChannelMonitorRuntime(ctx)
		cancel()
		if !runtime.ActiveProbesAllowed() {
			r.replaceScheduledMonitors(nil)
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), monitorStartupLoadTimeout)
	defer cancel()
	enabled, err := r.svc.ListEnabledMonitors(ctx)
	if err != nil {
		return err
	}
	r.replaceScheduledMonitors(enabled)
	return nil
}

// replaceScheduledMonitors 原子计算任务差异，锁外取消旧任务并启动新任务。
func (r *ChannelMonitorRunner) replaceScheduledMonitors(monitors []*ChannelMonitor) {
	desired := make(map[int64]*ChannelMonitor, len(monitors))
	for _, monitor := range monitors {
		if monitor == nil || !monitor.Enabled || monitor.IntervalSeconds <= 0 {
			continue
		}
		desired[monitor.ID] = monitor
	}

	var cancelled []*scheduledMonitor
	var started []*scheduledMonitor
	r.mu.Lock()
	if r.stopped || !r.started {
		r.mu.Unlock()
		return
	}
	for id, task := range r.tasks {
		monitor, keep := desired[id]
		if keep && task.name == monitor.Name && task.interval == time.Duration(monitor.IntervalSeconds)*time.Second {
			delete(desired, id)
			continue
		}
		delete(r.tasks, id)
		cancelled = append(cancelled, task)
	}
	for _, monitor := range desired {
		ctx, cancel := context.WithCancel(r.parentCtx)
		task := &scheduledMonitor{
			id:       monitor.ID,
			name:     monitor.Name,
			interval: time.Duration(monitor.IntervalSeconds) * time.Second,
			ctx:      ctx,
			cancel:   cancel,
		}
		r.tasks[monitor.ID] = task
		r.wg.Add(1)
		started = append(started, task)
	}
	r.mu.Unlock()

	for _, task := range cancelled {
		task.cancel()
	}
	for _, task := range started {
		go r.runScheduled(task.ctx, task)
	}
}

func (r *ChannelMonitorRunner) taskCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}

// Schedule 为指定监控创建（或重置）独立定时任务。
//   - m.Enabled=false → 等同于 Unschedule(m.ID)
//   - 已存在的任务会先被取消再重建（适用于 IntervalSeconds 变更场景）
//   - 新任务立即触发首次检测，之后按 IntervalSeconds 周期触发
func (r *ChannelMonitorRunner) Schedule(m *ChannelMonitor) {
	if r == nil || m == nil {
		return
	}
	if !m.Enabled {
		r.Unschedule(m.ID)
		return
	}
	if r.settingService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), monitorStartupLoadTimeout)
		runtime := r.settingService.GetChannelMonitorRuntime(ctx)
		cancel()
		if !runtime.ActiveProbesAllowed() {
			r.Unschedule(m.ID)
			return
		}
	}
	interval := time.Duration(m.IntervalSeconds) * time.Second
	if interval <= 0 {
		// Create/Update 已通过 validateInterval 校验区间，正常路径不可能到这里。
		// 真触发说明数据库中存在违反约束的数据或校验链路有 bug，记 Error 暴露问题。
		slog.Error("channel_monitor: skip schedule for invalid interval",
			"monitor_id", m.ID, "interval_seconds", m.IntervalSeconds)
		return
	}

	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	if !r.started {
		// Start 之前调用 Schedule 通常意味着 wire 顺序错乱：
		// 当前 wire 顺序是 SetScheduler → Start，CRUD 钩子最早也只能在请求到达时触发，
		// 此时 Start 早已完成。出现此分支时把 monitor 信息打出来便于排查，
		// 不入队、不缓存——交给运维通过重启或修复 wire 解决。
		r.mu.Unlock()
		slog.Warn("channel_monitor: schedule before runner started, skip",
			"monitor_id", m.ID, "name", m.Name)
		return
	}
	if existing, ok := r.tasks[m.ID]; ok {
		existing.cancel()
	}
	ctx, cancel := context.WithCancel(r.parentCtx)
	task := &scheduledMonitor{
		id:       m.ID,
		name:     m.Name,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
	r.tasks[m.ID] = task
	r.wg.Add(1)
	r.mu.Unlock()

	go r.runScheduled(ctx, task)
}

// Unschedule 取消指定监控的定时任务（若存在）。
// 已经在执行中的检测会通过 ctx 取消信号传递。
func (r *ChannelMonitorRunner) Unschedule(id int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	task, ok := r.tasks[id]
	if ok {
		delete(r.tasks, id)
	}
	r.mu.Unlock()
	if ok {
		task.cancel()
	}
}

// Stop 优雅停止：取消所有任务、关闭池。
func (r *ChannelMonitorRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	unsubscribe := r.unsub
	r.unsub = nil
	r.parentCancel()
	r.tasks = nil
	r.mu.Unlock()
	if unsubscribe != nil {
		unsubscribe()
	}

	r.wg.Wait()
	r.pool.StopAndWait()
}

// runScheduled 单个监控的循环：立即触发首次（满足"新建/启用即跑"），
// 之后按 interval 周期触发；ctx 取消即退出。
func (r *ChannelMonitorRunner) runScheduled(ctx context.Context, task *scheduledMonitor) {
	defer r.wg.Done()

	r.fire(ctx, task)

	ticker := time.NewTicker(task.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.fire(ctx, task)
		}
	}
}

// fire 提交一次检测到 worker 池。功能开关关闭时跳过本次（不取消任务，
// 重新启用时立即恢复）；池满或重复在飞时也跳过。
func (r *ChannelMonitorRunner) fire(ctx context.Context, task *scheduledMonitor) {
	if r.settingService != nil {
		rt := r.settingService.GetChannelMonitorRuntime(ctx)
		if !rt.ActiveProbesAllowed() {
			return
		}
	}
	if !r.tryAcquireInFlight(task.id) {
		slog.Debug("channel_monitor: skip already in-flight",
			"monitor_id", task.id, "name", task.name)
		return
	}
	if _, ok := r.pool.TrySubmit(func() {
		r.runOne(task.id, task.name)
	}); !ok {
		// 池满：丢弃本次检测，但必须释放已占用的 inFlight 槽，否则该 monitor 会被永久卡住。
		r.releaseInFlight(task.id)
		slog.Warn("channel_monitor: worker pool full, skip submission",
			"monitor_id", task.id, "name", task.name)
	}
}

// tryAcquireInFlight 原子地占用 monitor 的 in-flight 槽。
// 已被占用返回 false（调用方应跳过本次提交）。
func (r *ChannelMonitorRunner) tryAcquireInFlight(id int64) bool {
	r.inFlightMu.Lock()
	defer r.inFlightMu.Unlock()
	if _, exists := r.inFlight[id]; exists {
		return false
	}
	r.inFlight[id] = struct{}{}
	return true
}

// releaseInFlight 释放 in-flight 槽。runOne 完成（含 panic recover）后必须调用。
func (r *ChannelMonitorRunner) releaseInFlight(id int64) {
	r.inFlightMu.Lock()
	delete(r.inFlight, id)
	r.inFlightMu.Unlock()
}

// runOne 执行单个监控的检测。所有错误只记日志，不熔断。
// 任务结束时（含 panic recover）必须释放 in-flight 槽。
func (r *ChannelMonitorRunner) runOne(id int64, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), monitorRequestTimeout+monitorPingTimeout+monitorRunOneBuffer)
	defer cancel()

	defer r.releaseInFlight(id)

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("channel_monitor: runner panic",
				"monitor_id", id, "name", name, "panic", rec)
		}
	}()

	if _, err := r.svc.RunCheck(ctx, id); err != nil {
		slog.Warn("channel_monitor: run check failed",
			"monitor_id", id, "name", name, "error", err)
	}
}
