package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	BsPaySdk "github.com/huifurepo/bspay-go-sdk/BsPaySdk"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

const (
	huifuHostedPreOrderType = "1"
	huifuHostedUsageType    = "R"
	huifuProjectTitle       = "Wallet Top-up"
	huifuSDKVersion         = "php#v2.0.26"
)

var (
	huifuAPIBaseURL = strings.TrimRight(BsPaySdk.BASE_API_URL_V2, "/")
	huifuHTTPClient = http.DefaultClient
)

type HuifuConfig struct {
	SysID         string `json:"sys_id"`
	ProductID     string `json:"product_id"`
	MerchantID    string `json:"merchant_id"`
	ProjectID     string `json:"project_id"`
	SkillSource   string `json:"skill_source"`
	RSAPrivateKey string `json:"rsa_private_key"`
	RSAPublicKey  string `json:"rsa_public_key"`
	NotifyURL     string `json:"notify_url"`
}

type HuifuPreOrderParams struct {
	TradeNo     string
	ReqDate     string
	ReqSeqID    string
	Amount      string
	GoodsDesc   string
	CallbackURL string
}

type HuifuPreOrderResult struct {
	ReqDate    string
	ReqSeqID   string
	PreOrderID string
	JumpURL    string
	HfSeqID    string
	TransStat  string
	RawData    map[string]interface{}
}

type HuifuQueryResult struct {
	ReqSeqID  string
	HfSeqID   string
	TransStat string
	RawData   map[string]interface{}
}

type HuifuWebhookEnvelope struct {
	SysID     string          `json:"sys_id"`
	ProductID string          `json:"product_id"`
	Sign      string          `json:"sign"`
	Data      json.RawMessage `json:"data"`
	RespData  string          `json:"resp_data"`
}

type HuifuWebhookData struct {
	ReqDate   string `json:"req_date"`
	ReqSeqID  string `json:"req_seq_id"`
	HuifuID   string `json:"huifu_id"`
	HfSeqID   string `json:"hf_seq_id"`
	TransStat string `json:"trans_stat"`
}

type huifuSDKConfigFile struct {
	ProductID          string `json:"product_id"`
	SysID              string `json:"sys_id"`
	RSAMerchPrivateKey string `json:"rsa_merch_private_key"`
	RSAHuifuPublicKey  string `json:"rsa_huifu_public_key"`
}

func SaveHuifuConfig(ctx context.Context, cfg HuifuConfig) error {
	_ = ctx

	values := map[string]string{
		"HuifuSysID":       strings.TrimSpace(cfg.SysID),
		"HuifuProductID":   strings.TrimSpace(cfg.ProductID),
		"HuifuMerchantID":  strings.TrimSpace(cfg.MerchantID),
		"HuifuProjectID":   strings.TrimSpace(cfg.ProjectID),
		"HuifuSkillSource": strings.TrimSpace(cfg.SkillSource),
		"HuifuNotifyURL":   strings.TrimSpace(cfg.NotifyURL),
	}
	if pk := strings.TrimSpace(cfg.RSAPrivateKey); pk != "" {
		values["HuifuRSAPrivateKey"] = pk
	}
	if pub := strings.TrimSpace(cfg.RSAPublicKey); pub != "" {
		values["HuifuRSAPublicKey"] = pub
	}
	return model.UpdateOptionsBulk(values)
}

func BuildHuifuConfigFromSettings() HuifuConfig {
	return HuifuConfig{
		SysID:         strings.TrimSpace(setting.HuifuSysID),
		ProductID:     strings.TrimSpace(setting.HuifuProductID),
		MerchantID:    strings.TrimSpace(setting.HuifuMerchantID),
		ProjectID:     strings.TrimSpace(setting.HuifuProjectID),
		SkillSource:   strings.TrimSpace(setting.HuifuSkillSource),
		RSAPrivateKey: strings.TrimSpace(setting.HuifuRSAPrivateKey),
		RSAPublicKey:  strings.TrimSpace(setting.HuifuRSAPublicKey),
		NotifyURL:     strings.TrimSpace(setting.HuifuNotifyURL),
	}
}

func IsHuifuConfigComplete(cfg HuifuConfig) bool {
	return strings.TrimSpace(cfg.SysID) != "" &&
		strings.TrimSpace(cfg.ProductID) != "" &&
		strings.TrimSpace(cfg.MerchantID) != "" &&
		strings.TrimSpace(cfg.ProjectID) != "" &&
		strings.TrimSpace(cfg.RSAPrivateKey) != "" &&
		strings.TrimSpace(cfg.RSAPublicKey) != "" &&
		strings.TrimSpace(cfg.NotifyURL) != ""
}

func CreateHuifuHostedPreOrder(ctx context.Context, params HuifuPreOrderParams) (*HuifuPreOrderResult, error) {
	cfg := BuildHuifuConfigFromSettings()

	hostingData := map[string]interface{}{
		"project_title": huifuProjectTitle,
		"project_id":    cfg.ProjectID,
		"callback_url":  strings.TrimSpace(params.CallbackURL),
		"private_info":  strings.TrimSpace(params.TradeNo),
	}

	extendInfos := map[string]interface{}{
		"notify_url": cfg.NotifyURL,
		"usage_type": huifuHostedUsageType,
	}

	reqData := map[string]interface{}{
		"req_date":       strings.TrimSpace(params.ReqDate),
		"req_seq_id":     strings.TrimSpace(params.ReqSeqID),
		"huifu_id":       cfg.MerchantID,
		"trans_amt":      strings.TrimSpace(params.Amount),
		"goods_desc":     strings.TrimSpace(params.GoodsDesc),
		"pre_order_type": huifuHostedPreOrderType,
		"hosting_data":   mustMarshalJSONString(hostingData),
		"notify_url":     extendInfos["notify_url"],
		"usage_type":     extendInfos["usage_type"],
	}

	resp, err := postHuifuRequest(ctx, cfg, BsPaySdk.V2_TRADE_HOSTING_PAYMENT_PREORDER, reqData)
	if err != nil {
		return nil, fmt.Errorf("huifu preorder request failed: %w", err)
	}

	data := nestedMap(resp, "data")
	jumpURL := stringField(data, "jump_url")
	if jumpURL == "" {
		jumpURL = stringField(data, "pay_url")
	}
	return &HuifuPreOrderResult{
		ReqDate:    params.ReqDate,
		ReqSeqID:   params.ReqSeqID,
		PreOrderID: stringField(data, "pre_order_id"),
		JumpURL:    jumpURL,
		HfSeqID:    stringField(data, "hf_seq_id"),
		TransStat:  stringField(data, "trans_stat"),
		RawData:    data,
	}, nil
}

func QueryHuifuHostedOrder(ctx context.Context, orgReqDate, orgReqSeqID string) (*HuifuQueryResult, error) {
	cfg := BuildHuifuConfigFromSettings()

	reqData := map[string]interface{}{
		"req_date":       time.Now().Format("20060102"),
		"req_seq_id":     fmt.Sprintf("HUIFUQ%d", common.GetTimestamp()),
		"huifu_id":       cfg.MerchantID,
		"org_req_date":   strings.TrimSpace(orgReqDate),
		"org_req_seq_id": strings.TrimSpace(orgReqSeqID),
	}

	resp, err := postHuifuRequest(ctx, cfg, BsPaySdk.V2_TRADE_HOSTING_PAYMENT_QUERYORDERINFO, reqData)
	if err != nil {
		return nil, fmt.Errorf("huifu query request failed: %w", err)
	}

	data := nestedMap(resp, "data")
	return &HuifuQueryResult{
		ReqSeqID:  stringField(data, "req_seq_id"),
		HfSeqID:   stringField(data, "hf_seq_id"),
		TransStat: stringField(data, "trans_stat"),
		RawData:   data,
	}, nil
}

func VerifyHuifuWebhook(rawBody []byte) (*HuifuWebhookEnvelope, *HuifuWebhookData, error) {
	cfg := BuildHuifuConfigFromSettings()
	if !IsHuifuConfigComplete(cfg) {
		return nil, nil, fmt.Errorf("huifu config incomplete")
	}

	var envelope HuifuWebhookEnvelope
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return verifyHuifuWebhookForm(cfg, rawBody)
	}
	if strings.TrimSpace(envelope.Sign) == "" {
		return nil, nil, fmt.Errorf("huifu webhook missing sign or data")
	}
	if len(envelope.Data) == 0 && strings.TrimSpace(envelope.RespData) == "" {
		return nil, nil, fmt.Errorf("huifu webhook missing sign or data")
	}

	msc := buildHuifuMerchSysConfig(cfg)
	dataToVerify := string(envelope.Data)
	if strings.TrimSpace(envelope.RespData) != "" {
		dataToVerify = normalizeHuifuJSONString(strings.TrimSpace(envelope.RespData))
		envelope.Data = json.RawMessage(dataToVerify)
	}
	ok, err := BsPaySdk.RsaSignVerify(strings.TrimSpace(envelope.Sign), dataToVerify, msc)
	if err != nil {
		return nil, nil, fmt.Errorf("verify huifu webhook sign: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("verify huifu webhook sign failed")
	}

	var data HuifuWebhookData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, nil, fmt.Errorf("decode huifu webhook data: %w", err)
	}
	return &envelope, &data, nil
}

func verifyHuifuWebhookForm(cfg HuifuConfig, rawBody []byte) (*HuifuWebhookEnvelope, *HuifuWebhookData, error) {
	values, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return nil, nil, fmt.Errorf("decode huifu webhook form: %w", err)
	}

	sign := strings.TrimSpace(values.Get("sign"))
	respData := normalizeHuifuJSONString(strings.TrimSpace(values.Get("resp_data")))
	if sign == "" || respData == "" {
		return nil, nil, fmt.Errorf("huifu webhook missing sign or resp_data")
	}

	msc := buildHuifuMerchSysConfig(cfg)
	ok, err := BsPaySdk.RsaSignVerify(sign, respData, msc)
	if err != nil {
		return nil, nil, fmt.Errorf("verify huifu webhook sign: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("verify huifu webhook sign failed")
	}

	var data HuifuWebhookData
	if err := json.Unmarshal([]byte(respData), &data); err != nil {
		return nil, nil, fmt.Errorf("decode huifu webhook resp_data: %w", err)
	}

	return &HuifuWebhookEnvelope{
		SysID:     strings.TrimSpace(values.Get("sys_id")),
		ProductID: strings.TrimSpace(values.Get("product_id")),
		Sign:      sign,
		RespData:  respData,
		Data:      json.RawMessage(respData),
	}, &data, nil
}

func newHuifuSDK(cfg HuifuConfig) (*BsPaySdk.BsPay, func(), error) {
	if !IsHuifuConfigComplete(cfg) {
		return nil, func() {}, fmt.Errorf("huifu config incomplete")
	}

	dir, err := os.MkdirTemp("", "new-api-huifu-sdk-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create huifu temp dir: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	configPath := filepath.Join(dir, "config.json")
	payload := huifuSDKConfigFile{
		ProductID:          cfg.ProductID,
		SysID:              cfg.SysID,
		RSAMerchPrivateKey: cfg.RSAPrivateKey,
		RSAHuifuPublicKey:  cfg.RSAPublicKey,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("marshal huifu sdk config: %w", err)
	}
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("write huifu sdk config: %w", err)
	}

	sdk, err := BsPaySdk.NewBsPay(true, configPath)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("init huifu sdk: %w", err)
	}
	return sdk, cleanup, nil
}

func postHuifuRequest(ctx context.Context, cfg HuifuConfig, apiPath string, data map[string]interface{}) (map[string]interface{}, error) {
	if !IsHuifuConfigComplete(cfg) {
		return nil, fmt.Errorf("huifu config incomplete")
	}

	payloadData := deleteNilValues(data)
	msc := buildHuifuMerchSysConfig(cfg)

	dataText, err := formatHuifuJSON(payloadData)
	if err != nil {
		return nil, fmt.Errorf("marshal huifu request data: %w", err)
	}
	sign, err := BsPaySdk.RsaSign(dataText, msc)
	if err != nil {
		return nil, fmt.Errorf("sign huifu request: %w", err)
	}

	payload := map[string]interface{}{
		"sign":       sign,
		"sys_id":     cfg.SysID,
		"product_id": cfg.ProductID,
		"data":       payloadData,
	}
	bodyText, err := formatHuifuJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal huifu request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, huifuAPIBaseURL+apiPath, bytes.NewBufferString(bodyText))
	if err != nil {
		return nil, fmt.Errorf("build huifu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("charset", "UTF-8")
	req.Header.Set("sdk_version", huifuSDKVersion)
	if cfg.SkillSource != "" {
		req.Header.Set("jpt-x-skill-source", cfg.SkillSource)
		if huifuID := stringField(payloadData, "huifu_id"); huifuID != "" {
			req.Header.Set("jpt-x-skill-huifu_id", huifuID)
		}
	}

	resp, err := huifuHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read huifu response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huifu http status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("decode huifu response: %w body=%s", err, strings.TrimSpace(string(body)))
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		signValue := strings.TrimSpace(fmt.Sprintf("%v", respData["sign"]))
		rawData := raw["data"]
		if signValue != "" && len(rawData) > 0 {
			ok, verifyErr := BsPaySdk.RsaSignVerify(signValue, normalizeHuifuJSONString(string(rawData)), msc)
			if verifyErr != nil {
				return nil, fmt.Errorf("verify huifu response sign: %w", verifyErr)
			}
			if !ok {
				return nil, fmt.Errorf("verify huifu response sign failed")
			}
		}
	}

	return respData, nil
}

func buildHuifuMerchSysConfig(cfg HuifuConfig) *BsPaySdk.MerchSysConfig {
	return &BsPaySdk.MerchSysConfig{
		ProductId:          cfg.ProductID,
		SysId:              cfg.SysID,
		RsaMerchPrivateKey: cfg.RSAPrivateKey,
		RsaHuifuPublicKey:  cfg.RSAPublicKey,
	}
}

func deleteNilValues(src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(src))
	for key, value := range src {
		if key == "" || value == nil {
			continue
		}
		result[key] = value
	}
	return result
}

func formatHuifuJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return normalizeHuifuJSONString(string(data)), nil
}

func normalizeHuifuJSONString(raw string) string {
	raw = strings.ReplaceAll(raw, "\\u003c", "<")
	raw = strings.ReplaceAll(raw, "\\u003e", ">")
	raw = strings.ReplaceAll(raw, "\\u0026", "&")
	return raw
}

func mustMarshalJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func nestedMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	if val, ok := m[key]; ok {
		if data, ok := val.(map[string]interface{}); ok {
			return data
		}
	}
	return map[string]interface{}{}
}

func stringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	}
	return ""
}
