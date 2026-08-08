package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CompositeModelRoute 定义 Composite 分组的公开模型到上游平台/模型路由。
type CompositeModelRoute struct {
	ent.Schema
}

// Annotations 固定实体对应的数据库表名。
func (CompositeModelRoute) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "composite_model_routes"},
	}
}

// Mixin 为路由记录提供时间戳和软删除能力。
func (CompositeModelRoute) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Fields 定义模型匹配、目标平台和端点作用域等持久化字段。
func (CompositeModelRoute) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id").Comment("Composite 分组 ID"),
		field.String("public_model").MaxLen(200).NotEmpty().Comment("客户端公开模型标识或前缀"),
		field.String("match_type").MaxLen(20).Default("exact").Comment("匹配类型：exact 或 prefix"),
		field.String("target_platform").MaxLen(50).Default(domain.PlatformOpenAI).Comment("实际处理请求的上游平台"),
		field.String("upstream_model").MaxLen(200).Default("").Comment("上游模型标识；空值表示沿用公开模型"),
		field.String("endpoint").MaxLen(50).Default("any").Comment("路由生效的 API 端点范围"),
		field.Int("priority").Default(100).Comment("同等匹配强度下数值越小优先级越高"),
		field.Bool("enabled").Default(true).Comment("是否启用该路由"),
		field.String("notes").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}).Comment("管理备注"),
	}
}

// Edges 将路由记录关联到所属分组，并由数据库级联清理。
func (CompositeModelRoute) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", Group.Type).Unique().Required().Field("group_id"),
	}
}

// Indexes 定义常用筛选和排序索引；活动记录唯一约束由迁移中的部分索引实现。
func (CompositeModelRoute) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id"),
		index.Fields("group_id", "enabled"),
		index.Fields("group_id", "endpoint"),
		index.Fields("group_id", "target_platform"),
		index.Fields("deleted_at"),
		index.Fields("priority"),
	}
}
