import React, { useEffect, useState } from 'react';
import QRCode from 'qrcode';
import {
  Button,
  Card,
  Divider,
  Form,
  Grid,
  Header,
  Icon,
  Label,
  Message,
  Statistic,
  Table,
} from 'semantic-ui-react';
import { API, copy, showError, showInfo, showSuccess } from '../../helpers';
import { renderQuota } from '../../helpers/render';
import { useTranslation } from 'react-i18next';

const TopUp = () => {
  const { t } = useTranslation();
  const [packages, setPackages] = useState([]);
  const [orders, setOrders] = useState([]);
  const [quotaGrants, setQuotaGrants] = useState([]);
  const [redemptionCode, setRedemptionCode] = useState('');
  const [topUpLink, setTopUpLink] = useState('');
  const [userQuota, setUserQuota] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [loadingPackageId, setLoadingPackageId] = useState(0);
  const [activePayment, setActivePayment] = useState(null);
  const [paymentQrCode, setPaymentQrCode] = useState('');
  const [pollingOrderId, setPollingOrderId] = useState(0);
  const [paymentConfig, setPaymentConfig] = useState({});
  const [user, setUser] = useState({});

  const formatPrice = (priceCents) => {
    return `¥${(Number(priceCents || 0) / 100).toFixed(2)}`;
  };

  const formatTime = (timestamp) => {
    if (!timestamp || timestamp < 0) return '永久有效';
    return new Date(timestamp * 1000).toLocaleString();
  };

  const packageTypeText = (item) => {
    if (item.duration_days > 0) return `${item.duration_days} 天有效`;
    return '永久额度';
  };

  const orderStatusText = (status) => {
    switch (status) {
      case 1:
        return '待支付';
      case 2:
        return '已支付';
      case 3:
        return '已关闭';
      case 4:
        return '支付失败';
      default:
        return '未知';
    }
  };

  const loadUserQuota = async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) {
      setUserQuota(data.quota);
      setUser(data);
    } else {
      showError(message);
    }
  };

  const loadPackages = async () => {
    const res = await API.get('/api/package/available');
    const { success, message, data } = res.data;
    if (success) {
      setPackages(data || []);
    } else {
      showError(message);
    }
  };

  const loadOrders = async () => {
    const res = await API.get('/api/order/');
    const { success, message, data } = res.data;
    if (success) {
      setOrders(data || []);
    } else {
      showError(message);
    }
  };

  const loadQuotaGrants = async () => {
    const res = await API.get('/api/user/quota_grants');
    const { success, message, data } = res.data;
    if (success) {
      setQuotaGrants(data || []);
    } else {
      showError(message);
    }
  };

  const loadPaymentConfig = async () => {
    const res = await API.get('/api/order/payment_config');
    const { success, message, data } = res.data;
    if (success) {
      setPaymentConfig(data || {});
    } else {
      showError(message);
    }
  };

  const refreshData = async () => {
    await Promise.all([
      loadUserQuota(),
      loadPackages(),
      loadOrders(),
      loadQuotaGrants(),
      loadPaymentConfig(),
    ]);
  };

  const redeemCode = async () => {
    if (redemptionCode === '') {
      showInfo(t('topup.redeem_code.empty_code'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('topup.redeem_code.success'));
        setRedemptionCode('');
        refreshData().then();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('topup.redeem_code.request_failed'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('topup.redeem_code.no_link'));
      return;
    }
    const url = new URL(topUpLink);
    url.searchParams.append('username', user.username);
    url.searchParams.append('user_id', user.id);
    url.searchParams.append('transaction_id', crypto.randomUUID());
    window.open(url.toString(), '_blank');
  };

  const createOrder = async (packageId, provider) => {
    setLoadingPackageId(packageId);
    try {
      const res = await API.post('/api/order/', {
        package_id: packageId,
        provider,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setActivePayment(data);
      setPollingOrderId(data.id);
      showSuccess('订单已创建');
      if (data.payment_url && provider === 'alipay') {
        window.open(data.payment_url, '_blank');
      }
      loadOrders().then();
    } catch (err) {
      showError(err.message);
    } finally {
      setLoadingPackageId(0);
    }
  };

  const completeMockPayment = async () => {
    if (!activePayment || activePayment.provider !== 'mock') return;
    try {
      const res = await API.post(`/api/order/${activePayment.id}/mock_pay`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setActivePayment(data);
      setPollingOrderId(0);
      showSuccess('模拟支付成功，额度已到账');
      refreshData().then();
    } catch (err) {
      showError(err.message);
    }
  };

  useEffect(() => {
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      if (status.top_up_link) {
        setTopUpLink(status.top_up_link);
      }
    }
    refreshData().then();
  }, []);

  useEffect(() => {
    let cancelled = false;
    setPaymentQrCode('');
    if (
      activePayment &&
      activePayment.provider === 'wechat' &&
      activePayment.payment_url
    ) {
      QRCode.toDataURL(activePayment.payment_url, {
        width: 220,
        margin: 2,
        errorCorrectionLevel: 'M',
      })
        .then((url) => {
          if (!cancelled) {
            setPaymentQrCode(url);
          }
        })
        .catch((err) => {
          showError(`生成微信支付二维码失败：${err.message}`);
        });
    }
    return () => {
      cancelled = true;
    };
  }, [activePayment]);

  useEffect(() => {
    if (!pollingOrderId) return undefined;

    let stopped = false;
    const pollOrder = async () => {
      try {
        const res = await API.get(`/api/order/${pollingOrderId}`);
        const { success, data, message } = res.data;
        if (!success) {
          showError(message);
          setPollingOrderId(0);
          return;
        }
        setActivePayment((current) => {
          if (!current || current.id !== data.id) return current;
          return { ...current, ...data };
        });
        if (data.status === 2) {
          stopped = true;
          setPollingOrderId(0);
          showSuccess('支付成功，额度已到账');
          refreshData().then();
        } else if (data.status !== 1) {
          stopped = true;
          setPollingOrderId(0);
          loadOrders().then();
        }
      } catch (err) {
        showError(err.message);
        setPollingOrderId(0);
      }
    };

    pollOrder().then();
    const timer = setInterval(() => {
      if (!stopped) {
        pollOrder().then();
      }
    }, 3000);

    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [pollingOrderId]);

  return (
    <div className='dashboard-container'>
      <Grid stackable columns={2}>
        <Grid.Column width={5}>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h2'>充值中心</Header>
              </Card.Header>
              <Statistic style={{ marginTop: '1em' }}>
                <Statistic.Value style={{ color: '#2185d0' }}>
                  {renderQuota(userQuota, t)}
                </Statistic.Value>
                <Statistic.Label>当前可用额度</Statistic.Label>
              </Statistic>
              <Divider />
              <Button
                basic
                icon='refresh'
                content='刷新状态'
                onClick={refreshData}
              />
              {topUpLink ? (
                <Button
                  basic
                  icon='external alternate'
                  content='外部充值链接'
                  onClick={openTopUpLink}
                  style={{ marginLeft: '0.5em' }}
                />
              ) : null}
            </Card.Content>
          </Card>

          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h3'>
                  <Icon name='ticket alternate' />
                  兑换码充值
                </Header>
              </Card.Header>
              <Form.Input
                fluid
                icon='key'
                iconPosition='left'
                placeholder={t('topup.redeem_code.placeholder')}
                value={redemptionCode}
                onChange={(e) => {
                  setRedemptionCode(e.target.value);
                }}
                onPaste={(e) => {
                  e.preventDefault();
                  const pastedText = e.clipboardData.getData('text');
                  setRedemptionCode(pastedText.trim());
                }}
              />
              <Button
                color='green'
                fluid
                onClick={redeemCode}
                loading={isSubmitting}
                disabled={isSubmitting}
              >
                {isSubmitting
                  ? t('topup.redeem_code.submitting')
                  : t('topup.redeem_code.submit')}
              </Button>
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column width={11}>
          <Header as='h3'>购买套餐</Header>
          {packages.length === 0 ? (
            <Message info>管理员还没有上架可购买套餐。</Message>
          ) : (
            <Card.Group itemsPerRow={3} stackable>
              {packages.map((item) => (
                <Card key={item.id}>
                  <Card.Content>
                    <Card.Header>{item.name}</Card.Header>
                    <Card.Meta>{packageTypeText(item)}</Card.Meta>
                    <Card.Description style={{ minHeight: 70 }}>
                      {item.description || 'AI API 调用额度套餐'}
                    </Card.Description>
                    <Divider />
                    <Header as='h2' style={{ margin: 0 }}>
                      {formatPrice(item.price_cents)}
                    </Header>
                    <Label basic style={{ marginTop: '0.75em' }}>
                      {renderQuota(item.quota, t)}
                    </Label>
                  </Card.Content>
                  <Card.Content extra>
                    <Button
                      primary
                      fluid
                      onClick={() => createOrder(item.id, 'alipay')}
                      loading={loadingPackageId === item.id}
                      disabled={
                        loadingPackageId !== 0 ||
                        paymentConfig.alipay?.enabled === false
                      }
                    >
                      支付宝购买
                    </Button>
                    <Button
                      basic
                      fluid
                      style={{ marginTop: '0.5em' }}
                      onClick={() => createOrder(item.id, 'wechat')}
                      loading={loadingPackageId === item.id}
                      disabled={
                        loadingPackageId !== 0 ||
                        paymentConfig.wechat?.enabled === false
                      }
                    >
                      微信购买
                    </Button>
                    {paymentConfig.mock?.enabled ? (
                      <Button
                        basic
                        color='orange'
                        fluid
                        style={{ marginTop: '0.5em' }}
                        onClick={() => createOrder(item.id, 'mock')}
                        loading={loadingPackageId === item.id}
                        disabled={loadingPackageId !== 0}
                      >
                        模拟支付
                      </Button>
                    ) : null}
                  </Card.Content>
                </Card>
              ))}
            </Card.Group>
          )}

          {activePayment ? (
            <Message info style={{ marginTop: '1em' }}>
              <Message.Header>订单 {activePayment.order_no}</Message.Header>
              <p>
                {activePayment.provider === 'mock'
                  ? '这是本地模拟支付订单，点击下方按钮即可模拟支付成功并发放额度。'
                  : activePayment.provider === 'wechat'
                  ? '请使用微信扫码支付；支付完成后本页会自动刷新到账状态。'
                  : '支付宝支付页已打开；如果没有弹出窗口，可以复制下面的链接打开。'}
              </p>
              <p>当前状态：{orderStatusText(activePayment.status)}</p>
              {activePayment.provider === 'wechat' && paymentQrCode ? (
                <div style={{ textAlign: 'center', margin: '1em 0' }}>
                  <img
                    src={paymentQrCode}
                    alt='微信支付二维码'
                    style={{
                      width: 220,
                      height: 220,
                      border: '1px solid #e5e7eb',
                      padding: 8,
                      background: '#fff',
                    }}
                  />
                </div>
              ) : null}
              <Form.Input
                fluid
                value={activePayment.payment_url || ''}
                action={
                  <Button
                    icon='copy'
                    content='复制'
                    onClick={async () => {
                      if (await copy(activePayment.payment_url || '')) {
                        showSuccess('支付链接已复制');
                      }
                    }}
                  />
                }
              />
              <Button.Group style={{ marginTop: '0.75em' }}>
                {activePayment.provider === 'mock' &&
                activePayment.status === 1 ? (
                  <Button
                    color='orange'
                    icon='check circle'
                    content='完成模拟支付'
                    onClick={completeMockPayment}
                  />
                ) : null}
                {activePayment.provider === 'alipay' ? (
                  <Button
                    primary
                    icon='external alternate'
                    content='打开支付页'
                    onClick={() => window.open(activePayment.payment_url, '_blank')}
                  />
                ) : null}
                <Button
                  basic
                  icon='refresh'
                  content='立即检查到账'
                  onClick={async () => {
                    setPollingOrderId(activePayment.id);
                    const res = await API.get(`/api/order/${activePayment.id}`);
                    const { success, data, message } = res.data;
                    if (!success) {
                      showError(message);
                      return;
                    }
                    setActivePayment(data);
                    if (data.status === 2) {
                      showSuccess('支付成功，额度已到账');
                      refreshData().then();
                    } else {
                      showInfo('订单还未完成支付');
                    }
                  }}
                />
              </Button.Group>
            </Message>
          ) : null}

          <Header as='h3' style={{ marginTop: '1.5em' }}>
            我的订单
          </Header>
          <Table compact celled>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>订单号</Table.HeaderCell>
                <Table.HeaderCell>金额</Table.HeaderCell>
                <Table.HeaderCell>额度</Table.HeaderCell>
                <Table.HeaderCell>状态</Table.HeaderCell>
                <Table.HeaderCell>创建时间</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {orders.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan='5'>暂无订单</Table.Cell>
                </Table.Row>
              ) : (
                orders.map((order) => (
                  <Table.Row key={order.id}>
                    <Table.Cell>{order.order_no}</Table.Cell>
                    <Table.Cell>{formatPrice(order.amount_cents)}</Table.Cell>
                    <Table.Cell>{renderQuota(order.quota, t)}</Table.Cell>
                    <Table.Cell>{orderStatusText(order.status)}</Table.Cell>
                    <Table.Cell>{formatTime(order.created_time)}</Table.Cell>
                  </Table.Row>
                ))
              )}
            </Table.Body>
          </Table>

          <Header as='h3' style={{ marginTop: '1.5em' }}>
            额度包
          </Header>
          <Table compact celled>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>来源</Table.HeaderCell>
                <Table.HeaderCell>剩余额度</Table.HeaderCell>
                <Table.HeaderCell>总额度</Table.HeaderCell>
                <Table.HeaderCell>有效期</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {quotaGrants.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan='4'>暂无额度包记录</Table.Cell>
                </Table.Row>
              ) : (
                quotaGrants.map((grant) => (
                  <Table.Row key={grant.id}>
                    <Table.Cell>{grant.source_type}</Table.Cell>
                    <Table.Cell>{renderQuota(grant.remain_quota, t)}</Table.Cell>
                    <Table.Cell>{renderQuota(grant.quota, t)}</Table.Cell>
                    <Table.Cell>{formatTime(grant.expired_time)}</Table.Cell>
                  </Table.Row>
                ))
              )}
            </Table.Body>
          </Table>
        </Grid.Column>
      </Grid>
    </div>
  );
};

export default TopUp;
