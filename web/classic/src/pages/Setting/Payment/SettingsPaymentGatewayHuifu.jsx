/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { BookOpen } from 'lucide-react';

const defaultInputs = {
  HuifuSysID: '',
  HuifuProductID: '',
  HuifuMerchantID: '',
  HuifuProjectID: '',
  HuifuSkillSource: '',
  HuifuNotifyURL: '',
  HuifuRSAPrivateKey: '',
  HuifuRSAPublicKey: '',
};

export default function SettingsPaymentGatewayHuifu(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('Huifu 设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(defaultInputs);
  const formApiRef = useRef(null);

  useEffect(() => {
    if (!props.options || !formApiRef.current) return;

    const currentInputs = {
      HuifuSysID: props.options.HuifuSysID || '',
      HuifuProductID: props.options.HuifuProductID || '',
      HuifuMerchantID: props.options.HuifuMerchantID || '',
      HuifuProjectID: props.options.HuifuProjectID || '',
      HuifuSkillSource: props.options.HuifuSkillSource || '',
      HuifuNotifyURL: props.options.HuifuNotifyURL || '',
      HuifuRSAPrivateKey: '',
      HuifuRSAPublicKey: '',
    };

    setInputs(currentInputs);
    formApiRef.current.setValues(currentInputs);
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitHuifuSetting = async () => {
    const values = {
      ...inputs,
      ...(formApiRef.current?.getValues?.() || {}),
    };

    setLoading(true);
    try {
      const res = await API.post('/api/option/huifu/save', {
        sys_id: (values.HuifuSysID || '').trim(),
        product_id: (values.HuifuProductID || '').trim(),
        merchant_id: (values.HuifuMerchantID || '').trim(),
        project_id: (values.HuifuProjectID || '').trim(),
        skill_source: (values.HuifuSkillSource || '').trim(),
        notify_url: (values.HuifuNotifyURL || '').trim(),
        rsa_private_key: (values.HuifuRSAPrivateKey || '').trim(),
        rsa_public_key: (values.HuifuRSAPublicKey || '').trim(),
      });

      if (res.data?.success || res.data?.message === 'success') {
        showSuccess(t('更新成功'));
        props.refresh?.();
      } else {
        const message =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('更新失败');
        showError(message);
      }
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  const webhookUrl = props.options?.ServerAddress
    ? `${removeTrailingSlash(props.options.ServerAddress)}/api/huifu/webhook`
    : `${t('网站地址')}/api/huifu/webhook`;

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<BookOpen size={16} />}
            description={
              <>
                {t(
                  '用于配置汇付 H5/PC 收银台充值。notify_url 必须是公网可访问地址，callback_url 仅负责把用户带回前端页面，最终支付成功仍以后端异步通知与查单确认为准。',
                )}
                <br />
                {t('推荐 webhook 地址')}：{webhookUrl}
              </>
            }
            style={{ marginBottom: 16 }}
          />

          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuSysID'
                label='HuifuSysID'
                placeholder={t('外层请求 sys_id')}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuProductID'
                label='HuifuProductID'
                placeholder={t('外层请求 product_id')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuMerchantID'
                label='HuifuMerchantID'
                placeholder={t('业务参数 data.huifu_id')}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuProjectID'
                label='HuifuProjectID'
                placeholder={t('托管参数 hosting_data.project_id')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuSkillSource'
                label='HuifuSkillSource'
                placeholder={t('可选的来源标识')}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='HuifuNotifyURL'
                label='HuifuNotifyURL'
                placeholder={t('公网可访问的 notify_url')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24}>
              <Form.TextArea
                field='HuifuRSAPrivateKey'
                label='HuifuRSAPrivateKey'
                placeholder={t('留空表示保持当前已保存的私钥不变')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24}>
              <Form.TextArea
                field='HuifuRSAPublicKey'
                label='HuifuRSAPublicKey'
                placeholder={t('留空表示保持当前已保存的公钥不变')}
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
          </Row>

          <Button onClick={submitHuifuSetting} style={{ marginTop: 16 }}>
            {t('更新 Huifu 设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
