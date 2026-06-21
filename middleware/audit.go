package middleware

import (
	"bytes"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// auditResponseWriter 包装 gin.ResponseWriter，捕获响应状态码并将响应体复制一份到
// 有限大小的缓冲区，用于判断业务是否成功（解析响应 JSON 的 success 字段）。
// 缓冲区有上限，避免大响应（如密钥导出）占用过多内存；超出上限则不再缓存，
// 此时仅依据 HTTP 状态码判断成败。
type auditResponseWriter struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	maxSize int
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	if w.body.Len() < w.maxSize {
		remain := w.maxSize - w.body.Len()
		if remain >= len(b) {
			w.body.Write(b)
		} else {
			w.body.Write(b[:remain])
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *auditResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// auditRouteActions 将「METHOD + 路由模板」映射为语言无关的操作标识 action。
// 这些是未被 handler 手动埋点的写操作，由中间件兜底记录；前端依据 action 用 i18n 本地化展示。
// 未命中的写操作回退为 action="generic"，前端展示 "METHOD route"。
var auditRouteActions = map[string]string{
	// 用户管理
	"POST /api/user/topup/complete":                    "user.topup_complete",
	"DELETE /api/user/:id/reset_passkey":               "user.reset_passkey",
	"DELETE /api/user/:id/oauth/bindings/:provider_id": "user.oauth_unbind",

	// 系统设置（root）
	"POST /api/option/payment_compliance":       "option.payment_compliance",
	"POST /api/option/rest_model_ratio":         "option.reset_ratio",
	"DELETE /api/option/channel_affinity_cache": "option.clear_affinity_cache",

	// 自定义 OAuth（root）
	"POST /api/custom-oauth-provider/":      "custom_oauth.create",
	"PUT /api/custom-oauth-provider/:id":    "custom_oauth.update",
	"DELETE /api/custom-oauth-provider/:id": "custom_oauth.delete",

	// 性能/缓存（root）
	"DELETE /api/performance/disk_cache": "performance.clear_disk_cache",
	"POST /api/performance/gc":           "performance.gc",
	"DELETE /api/performance/logs":       "performance.clear_logs",

	// 兑换码
	"PUT /api/redemption/":           "redemption.update",
	"DELETE /api/redemption/:id":     "redemption.delete",
	"POST /api/redemption/batch":     "redemption.delete_batch",
	"DELETE /api/redemption/invalid": "redemption.delete_invalid",

	// 预填组
	"POST /api/prefill_group/":      "prefill_group.create",
	"PUT /api/prefill_group/":       "prefill_group.update",
	"DELETE /api/prefill_group/:id": "prefill_group.delete",

	// 供应商
	"POST /api/vendors/":      "vendor.create",
	"PUT /api/vendors/":       "vendor.update",
	"DELETE /api/vendors/:id": "vendor.delete",

	// 模型元数据
	"POST /api/models/":      "model.create",
	"PUT /api/models/":       "model.update",
	"DELETE /api/models/:id": "model.delete",
}

// AuditLogger 记录后台管理类变更操作的审计日志。
// 约定：
// - 仅在已登录且携带用户 ID 时记录；匿名请求忽略。
// - 优先使用路由模板（FullPath）而非原始 URL，便于聚合。
// - 对业务失败不记录，避免噪音；成功判定优先看响应体中的 success 字段，其次 HTTP 2xx。
func AuditLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			c.Next()
			return
		}

		uid, exists := c.Get(string(constant.ContextKeyUserId))
		if !exists {
			c.Next()
			return
		}
		userID, ok := uid.(int)
		if !ok || userID <= 0 {
			c.Next()
			return
		}

		auditWriter := beginAdminAudit(c)
		c.Next()
		finishAdminAudit(c, auditWriter)

		status := c.Writer.Status()
		if status < 200 || status >= 300 {
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		action := auditRouteActions[method+" "+path]
		if action == "" {
			return
		}

		detail := map[string]interface{}{
			"method": method,
			"route":  path,
		}

		content := auditContentEN(action, detail)
		operatorInfo := auditOperatorInfo(c)
		gopool.Go(func() {
			model.RecordOperationAuditLog(userID, content, c.ClientIP(), action, detail, operatorInfo, nil)
		})
	}
}

func beginAdminAudit(c *gin.Context) *auditResponseWriter {
	if c == nil {
		return nil
	}
	if _, exists := c.Get(string(constant.ContextKeyAuditLogged)); exists {
		return nil
	}
	aw := &auditResponseWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
		maxSize:        32 * 1024,
	}
	c.Writer = aw
	return aw
}

func finishAdminAudit(c *gin.Context, aw *auditResponseWriter) {
	if c == nil || aw == nil {
		return
	}
	if common.GetContextKeyBool(c, constant.ContextKeyAuditLogged) {
		return
	}
	status := c.Writer.Status()
	if status < 200 || status >= 300 {
		return
	}
	if !isAPISuccessPayload(aw.body.Bytes()) {
		return
	}
	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	action := auditRouteActions[c.Request.Method+" "+path]
	if action == "" {
		action = "generic"
	}
	detail := map[string]interface{}{
		"method": c.Request.Method,
		"route":  path,
	}
	content := auditContentEN(action, detail)
	operatorInfo := auditOperatorInfo(c)
	gopool.Go(func() {
		model.RecordOperationAuditLog(c.GetInt("id"), content, c.ClientIP(), action, detail, operatorInfo, nil)
	})
}

func isAPISuccessPayload(payload []byte) bool {
	if len(bytes.TrimSpace(payload)) == 0 {
		return true
	}
	var data struct {
		Success *bool `json:"success"`
	}
	if err := common.Unmarshal(payload, &data); err != nil {
		return true
	}
	if data.Success == nil {
		return true
	}
	return *data.Success
}

func auditContentEN(action string, params map[string]interface{}) string {
	templates := map[string]string{
		"user.topup_complete":                "Completed user top-up",
		"user.reset_passkey":                 "Reset the user passkey",
		"user.oauth_unbind":                  "Unbound user OAuth provider",
		"option.payment_compliance":          "Updated payment compliance settings",
		"option.reset_ratio":                 "Reset model ratio settings",
		"option.clear_affinity_cache":        "Cleared channel affinity cache",
		"custom_oauth.create":                "Created custom OAuth provider",
		"custom_oauth.update":                "Updated custom OAuth provider",
		"custom_oauth.delete":                "Deleted custom OAuth provider",
		"performance.clear_disk_cache":       "Cleared disk cache",
		"performance.gc":                     "Triggered garbage collection",
		"performance.clear_logs":             "Cleared logs",
		"redemption.update":                  "Updated redemption code",
		"redemption.delete":                  "Deleted redemption code",
		"redemption.delete_batch":            "Batch deleted redemption codes",
		"redemption.delete_invalid":          "Deleted invalid redemption codes",
		"prefill_group.create":               "Created prefill group",
		"prefill_group.update":               "Updated prefill group",
		"prefill_group.delete":               "Deleted prefill group",
		"vendor.create":                      "Created vendor",
		"vendor.update":                      "Updated vendor",
		"vendor.delete":                      "Deleted vendor",
		"model.create":                       "Created model metadata",
		"model.update":                       "Updated model metadata",
		"model.delete":                       "Deleted model metadata",
		"generic":                            "Performed administrative write operation",
	}
	tmpl, ok := templates[action]
	if !ok {
		return action
	}
	return os.Expand(tmpl, func(key string) string {
		if v, ok := params[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	})
}

func auditOperatorInfo(c *gin.Context) map[string]interface{} {
	if c == nil {
		return nil
	}
	return map[string]interface{}{
		"admin_id":       c.GetInt("id"),
		"admin_username": c.GetString("username"),
		"admin_role":     c.GetInt("role"),
		"auth_method":    auditAuthMethod(c),
	}
}

func auditAuthMethod(c *gin.Context) string {
	if c != nil && c.GetBool("use_access_token") {
		return "access_token"
	}
	return "session"
}
