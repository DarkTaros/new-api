package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type HuifuPayRequest struct {
	Amount int64 `json:"amount"`
}

type huifuConfirmRequest struct {
	TradeNo string `json:"trade_no"`
}

type saveHuifuConfigRequest struct {
	SysID         string `json:"sys_id"`
	ProductID     string `json:"product_id"`
	MerchantID    string `json:"merchant_id"`
	ProjectID     string `json:"project_id"`
	SkillSource   string `json:"skill_source"`
	RSAPrivateKey string `json:"rsa_private_key"`
	RSAPublicKey  string `json:"rsa_public_key"`
	NotifyURL     string `json:"notify_url"`
}

func RequestHuifuAmount(c *gin.Context) {
	if !isHuifuTopUpEnabled() {
		common.ApiErrorMsg(c, "汇付收银台未启用")
		return
	}

	var req HuifuPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < int64(setting.HuifuMinTopUp) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", setting.HuifuMinTopUp))
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}

	payMoney := getPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	common.ApiSuccess(c, fmt.Sprintf("%.2f", payMoney))
}

func RequestHuifuPay(c *gin.Context) {
	if !isHuifuTopUpEnabled() {
		common.ApiErrorMsg(c, "汇付收银台未启用")
		return
	}

	var req HuifuPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < int64(setting.HuifuMinTopUp) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", setting.HuifuMinTopUp))
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	payMoney := getPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	tradeNo := fmt.Sprintf("HFU%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	reqDate := time.Now().Format("20060102")
	reqSeqID := tradeNo
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          normalizeWaffoPancakeTopUpAmount(req.Amount),
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodHuifu,
		PaymentProvider: model.PaymentProviderHuifu,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	callbackURL := paymentReturnPath("/console/topup?show_history=true")
	result, err := service.CreateHuifuHostedPreOrder(c.Request.Context(), service.HuifuPreOrderParams{
		TradeNo:     tradeNo,
		ReqDate:     reqDate,
		ReqSeqID:    reqSeqID,
		Amount:      formatWaffoPancakeAmount(payMoney),
		GoodsDesc:   "Wallet top-up",
		CallbackURL: callbackURL,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 预下单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	if err := topUp.SetProviderMeta(model.HuifuTopUpMeta{
		ReqDate:    reqDate,
		ReqSeqID:   reqSeqID,
		PreOrderID: result.PreOrderID,
		JumpURL:    result.JumpURL,
		HfSeqID:    result.HfSeqID,
		TransStat:  result.TransStat,
	}); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 保存订单元数据失败 trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "保存订单失败")
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Huifu 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f jump_url=%q", id, tradeNo, req.Amount, payMoney, result.JumpURL))
	common.ApiSuccess(c, gin.H{
		"jump_url":     result.JumpURL,
		"trade_no":     tradeNo,
		"req_seq_id":   reqSeqID,
		"pre_order_id": result.PreOrderID,
	})
}

func HuifuWebhook(c *gin.Context) {
	if !isHuifuWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Huifu webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.Status(http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "invalid body")
		return
	}
	querySign := strings.TrimSpace(c.Query("sign"))
	if querySign != "" && !strings.Contains(string(body), "sign=") {
		if len(body) > 0 {
			body = append([]byte("sign="+url.QueryEscape(querySign)+"&"), body...)
		} else {
			body = []byte("sign=" + url.QueryEscape(querySign))
		}
	}
	if len(body) == 0 {
		if err := c.Request.ParseForm(); err == nil && len(c.Request.PostForm) > 0 {
			body = []byte(c.Request.PostForm.Encode())
		}
	}

	_, payload, err := service.VerifyHuifuWebhook(body)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Huifu webhook 验签失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "invalid sign")
		return
	}

	tradeNo := strings.TrimSpace(payload.ReqSeqID)
	if tradeNo == "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Huifu webhook 缺少 req_seq_id client_ip=%s", c.ClientIP()))
		c.String(http.StatusBadRequest, "missing req_seq_id")
		return
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderHuifu {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Huifu webhook 未找到本地订单 trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		c.String(http.StatusOK, "RECV_ORD_ID_"+tradeNo)
		return
	}

	meta, _ := topUp.GetHuifuMeta()
	if meta.ReqDate == "" {
		meta.ReqDate = payload.ReqDate
	}
	if meta.ReqSeqID == "" {
		meta.ReqSeqID = payload.ReqSeqID
	}
	meta.HfSeqID = payload.HfSeqID
	meta.TransStat = payload.TransStat
	_ = topUp.SetProviderMeta(meta)

	query, err := service.QueryHuifuHostedOrder(c.Request.Context(), meta.ReqDate, meta.ReqSeqID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu webhook 查单失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		c.String(http.StatusInternalServerError, "query failed")
		return
	}

	meta.HfSeqID = firstNonEmpty(query.HfSeqID, meta.HfSeqID)
	meta.TransStat = firstNonEmpty(query.TransStat, payload.TransStat, meta.TransStat)
	_ = topUp.SetProviderMeta(meta)

	switch meta.TransStat {
	case "S":
		if err := model.RechargeHuifu(tradeNo, c.ClientIP()); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 入账失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
			c.String(http.StatusInternalServerError, "recharge failed")
			return
		}
	case "F":
		if err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderHuifu, common.TopUpStatusExpired); err != nil && err != model.ErrTopUpStatusInvalid && err != model.ErrTopUpNotFound {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 标记失败订单失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		}
	case "P", "I":
	default:
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Huifu webhook 收到未知状态 trade_no=%s trans_stat=%s client_ip=%s", tradeNo, meta.TransStat, c.ClientIP()))
	}

	c.String(http.StatusOK, "RECV_ORD_ID_"+payload.ReqSeqID)
}

func ConfirmHuifuTopUp(c *gin.Context) {
	var req huifuConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	tradeNo := strings.TrimSpace(req.TradeNo)
	if tradeNo == "" {
		common.ApiErrorMsg(c, "缺少 trade_no")
		return
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderHuifu {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}

	meta, err := topUp.GetHuifuMeta()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 读取订单元数据失败 trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "订单状态异常")
		return
	}
	if strings.TrimSpace(meta.ReqDate) == "" || strings.TrimSpace(meta.ReqSeqID) == "" {
		common.ApiErrorMsg(c, "订单缺少查单锚点")
		return
	}

	query, err := service.QueryHuifuHostedOrder(c.Request.Context(), meta.ReqDate, meta.ReqSeqID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 主动查单失败 trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "查单失败")
		return
	}

	meta.HfSeqID = firstNonEmpty(query.HfSeqID, meta.HfSeqID)
	meta.TransStat = firstNonEmpty(query.TransStat, meta.TransStat)
	if err := topUp.SetProviderMeta(meta); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 更新订单元数据失败 trade_no=%s error=%q", tradeNo, err.Error()))
		common.ApiErrorMsg(c, "更新订单失败")
		return
	}

	switch meta.TransStat {
	case "S":
		if err := model.RechargeHuifu(tradeNo, c.ClientIP()); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 主动补单入账失败 trade_no=%s error=%q", tradeNo, err.Error()))
			common.ApiErrorMsg(c, "入账失败")
			return
		}
	case "F":
		if err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderHuifu, common.TopUpStatusExpired); err != nil && err != model.ErrTopUpStatusInvalid && err != model.ErrTopUpNotFound {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 主动补单标记失败失败 trade_no=%s error=%q", tradeNo, err.Error()))
		}
	case "P", "I":
	default:
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Huifu 主动查单收到未知状态 trade_no=%s trans_stat=%s", tradeNo, meta.TransStat))
	}

	common.ApiSuccess(c, gin.H{
		"trade_no":   tradeNo,
		"trans_stat": meta.TransStat,
		"status":     model.GetTopUpByTradeNo(tradeNo).Status,
		"hf_seq_id":  meta.HfSeqID,
	})
}

func SaveHuifuConfig(c *gin.Context) {
	var req saveHuifuConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL != "" {
		parsed, err := url.ParseRequestURI(notifyURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			common.ApiErrorMsg(c, "Notify URL 格式错误")
			return
		}
	}

	if err := service.SaveHuifuConfig(c.Request.Context(), service.HuifuConfig{
		SysID:         req.SysID,
		ProductID:     req.ProductID,
		MerchantID:    req.MerchantID,
		ProjectID:     req.ProjectID,
		SkillSource:   req.SkillSource,
		RSAPrivateKey: req.RSAPrivateKey,
		RSAPublicKey:  req.RSAPublicKey,
		NotifyURL:     notifyURL,
	}); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Huifu 保存配置失败 sys_id=%q product_id=%q merchant_id=%q error=%q", req.SysID, req.ProductID, req.MerchantID, err.Error()))
		common.ApiErrorMsg(c, "保存配置失败")
		return
	}

	common.ApiSuccess(c, gin.H{
		"sys_id":       setting.HuifuSysID,
		"product_id":   setting.HuifuProductID,
		"merchant_id":  setting.HuifuMerchantID,
		"project_id":   setting.HuifuProjectID,
		"skill_source": setting.HuifuSkillSource,
		"notify_url":   setting.HuifuNotifyURL,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
