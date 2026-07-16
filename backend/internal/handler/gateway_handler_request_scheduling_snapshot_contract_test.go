package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGatewayHandlerFailoverPaths_InstallRequestSchedulingSnapshot 约束所有会在同一请求内重试选号的
// OpenAI 兼容入口，必须在首次 SelectAccountWithLoadAwareness 前创建请求级调度快照。
// 该契约防止未来重构时把快照遗漏在 Responses 或 Chat Completions 的故障转移循环之外。
func TestGatewayHandlerFailoverPaths_InstallRequestSchedulingSnapshot(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	handlerDir := filepath.Dir(currentFile)

	for _, tc := range []struct {
		name     string
		fileName string
		method   string
	}{
		{name: "responses", fileName: "gateway_handler_responses.go", method: "Responses"},
		{name: "chat_completions", fileName: "gateway_handler_chat_completions.go", method: "ChatCompletions"},
		{name: "anthropic_messages", fileName: "gateway_handler.go", method: "Messages"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, filepath.Join(handlerDir, tc.fileName), nil, 0)
			require.NoError(t, err)

			method := findHandlerMethod(file, tc.method)
			require.NotNil(t, method, "必须保留 %s handler", tc.method)

			snapshotPos, firstSelectionPos := requestSchedulingCallPositions(method)
			require.NotEqual(t, token.NoPos, snapshotPos,
				"%s 必须在 failover 前调用 WithRequestSchedulingSnapshot", tc.method)
			require.NotEqual(t, token.NoPos, firstSelectionPos,
				"%s 必须保留 SelectAccountWithLoadAwareness 故障转移入口", tc.method)
			require.Less(t, int(snapshotPos), int(firstSelectionPos),
				"%s 必须在首次选号前创建请求级快照", tc.method)
		})
	}
}

// findHandlerMethod 从单个 handler 源文件中定位指定的方法声明，避免依赖文件行号。
func findHandlerMethod(file *ast.File, methodName string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if ok && method.Name != nil && method.Name.Name == methodName {
			return method
		}
	}
	return nil
}

// requestSchedulingCallPositions 返回快照创建及首次选号调用的位置，用于校验二者的执行顺序。
func requestSchedulingCallPositions(method *ast.FuncDecl) (snapshotPos token.Pos, firstSelectionPos token.Pos) {
	if method == nil || method.Body == nil {
		return token.NoPos, token.NoPos
	}

	ast.Inspect(method.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil {
			return true
		}
		switch selector.Sel.Name {
		case "WithRequestSchedulingSnapshot":
			if snapshotPos == token.NoPos {
				snapshotPos = call.Pos()
			}
		case "SelectAccountWithLoadAwareness":
			if firstSelectionPos == token.NoPos {
				firstSelectionPos = call.Pos()
			}
		}
		return true
	})
	return snapshotPos, firstSelectionPos
}
