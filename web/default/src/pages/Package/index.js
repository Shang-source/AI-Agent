import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Divider,
  Form,
  Grid,
  Header,
  Label,
  Message,
  Table,
} from 'semantic-ui-react';
import { API, showError, showSuccess } from '../../helpers';
import { renderQuota } from '../../helpers/render';
import { useTranslation } from 'react-i18next';

const emptyPackage = {
  id: 0,
  name: '',
  description: '',
  type: 'fixed',
  price_cents: 990,
  quota: 500000,
  duration_days: 0,
  status: 1,
  sort: 0,
};

const Package = () => {
  const { t } = useTranslation();
  const [packages, setPackages] = useState([]);
  const [orders, setOrders] = useState([]);
  const [paymentStatus, setPaymentStatus] = useState({});
  const [inputs, setInputs] = useState(emptyPackage);
  const [orderFilters, setOrderFilters] = useState({
    keyword: '',
    user_id: '',
    provider: '',
    status: 0,
  });
  const [orderPage, setOrderPage] = useState(0);
  const [loading, setLoading] = useState(false);

  const formatPrice = (priceCents) => {
    return `¥${(Number(priceCents || 0) / 100).toFixed(2)}`;
  };

  const formatTime = (timestamp) => {
    if (!timestamp) return '';
    return new Date(timestamp * 1000).toLocaleString();
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
      case 5:
        return '已退款';
      default:
        return '未知';
    }
  };

  const orderStatusColor = (status) => {
    switch (status) {
      case 1:
        return 'yellow';
      case 2:
        return 'green';
      case 3:
        return 'grey';
      case 4:
        return 'red';
      case 5:
        return 'orange';
      default:
        return 'grey';
    }
  };

  const loadPackages = async () => {
    const res = await API.get('/api/package/');
    const { success, message, data } = res.data;
    if (success) {
      setPackages(data || []);
    } else {
      showError(message);
    }
  };

  const loadOrders = async (page = orderPage, filters = orderFilters) => {
    const res = await API.get('/api/order/admin/all', {
      params: {
        p: page,
        keyword: filters.keyword,
        user_id: filters.user_id,
        provider: filters.provider,
        status: filters.status,
      },
    });
    const { success, message, data } = res.data;
    if (success) {
      setOrders(data || []);
      setOrderPage(page);
    } else {
      showError(message);
    }
  };

  const loadPaymentStatus = async () => {
    const res = await API.get('/api/order/admin/payment_config');
    const { success, message, data } = res.data;
    if (success) {
      setPaymentStatus(data || {});
    } else {
      showError(message);
    }
  };

  const refresh = async () => {
    await Promise.all([loadPackages(), loadOrders(), loadPaymentStatus()]);
  };

  const handleInputChange = (e, { name, value }) => {
    setInputs({ ...inputs, [name]: value });
  };

  const handleOrderFilterChange = (e, { name, value }) => {
    setOrderFilters({ ...orderFilters, [name]: value });
  };

  const searchOrders = async () => {
    await loadOrders(0, orderFilters);
  };

  const resetOrderFilters = async () => {
    const filters = {
      keyword: '',
      user_id: '',
      provider: '',
      status: 0,
    };
    setOrderFilters(filters);
    await loadOrders(0, filters);
  };

  const submitPackage = async () => {
    setLoading(true);
    try {
      const payload = {
        ...inputs,
        id: Number(inputs.id),
        price_cents: Number(inputs.price_cents),
        quota: Number(inputs.quota),
        duration_days: Number(inputs.duration_days),
        status: Number(inputs.status),
        sort: Number(inputs.sort),
      };
      const res = payload.id
        ? await API.put('/api/package/', payload)
        : await API.post('/api/package/', payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(payload.id ? '套餐已更新' : '套餐已创建');
        setInputs(emptyPackage);
        loadPackages().then();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const deletePackage = async (id) => {
    if (!window.confirm('确认删除该套餐？')) return;
    const res = await API.delete(`/api/package/${id}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('套餐已删除');
      loadPackages().then();
    } else {
      showError(message);
    }
  };

  const markPaid = async (id) => {
    if (!window.confirm('确认将该订单标记为已支付并发放额度？')) return;
    const res = await API.post(`/api/order/admin/${id}/mark_paid`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('订单已补单到账');
      loadOrders().then();
    } else {
      showError(message);
    }
  };

  const closeOrder = async (id) => {
    if (!window.confirm('确认关闭该待支付订单？')) return;
    const res = await API.post(`/api/order/admin/${id}/close`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('订单已关闭');
      loadOrders().then();
    } else {
      showError(message);
    }
  };

  const refundOrder = async (id) => {
    const reason = window.prompt('请输入退款/作废原因（可选）', '');
    if (reason === null) return;
    if (!window.confirm('确认退款作废该订单？系统会扣回该订单额度并作废对应额度记录。')) return;
    const res = await API.post(`/api/order/admin/${id}/refund`, { reason });
    const { success, message } = res.data;
    if (success) {
      showSuccess('订单已退款作废');
      loadOrders().then();
    } else {
      showError(message);
    }
  };

  useEffect(() => {
    refresh().then();
  }, []);

  return (
    <div className='dashboard-container'>
      <Grid stackable columns={2}>
        <Grid.Column width={5}>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h2'>套餐管理</Header>
              </Card.Header>
              <Form loading={loading}>
                <Form.Input
                  label='套餐名称'
                  name='name'
                  value={inputs.name}
                  onChange={handleInputChange}
                />
                <Form.TextArea
                  label='套餐说明'
                  name='description'
                  value={inputs.description}
                  onChange={handleInputChange}
                />
                <Form.Select
                  label='套餐类型'
                  name='type'
                  value={inputs.type}
                  options={[
                    { key: 'fixed', text: '固定额度包', value: 'fixed' },
                    { key: 'duration', text: '限期额度包', value: 'duration' },
                  ]}
                  onChange={handleInputChange}
                />
                <Form.Group widths='equal'>
                  <Form.Input
                    label='价格（分）'
                    name='price_cents'
                    type='number'
                    min='1'
                    value={inputs.price_cents}
                    onChange={handleInputChange}
                  />
                  <Form.Input
                    label={`额度${renderQuota(inputs.quota, t) ? ` (${renderQuota(inputs.quota, t)})` : ''}`}
                    name='quota'
                    type='number'
                    min='1'
                    value={inputs.quota}
                    onChange={handleInputChange}
                  />
                </Form.Group>
                <Form.Group widths='equal'>
                  <Form.Input
                    label='有效天数'
                    name='duration_days'
                    type='number'
                    min='0'
                    value={inputs.duration_days}
                    onChange={handleInputChange}
                    disabled={inputs.type === 'fixed'}
                  />
                  <Form.Input
                    label='排序'
                    name='sort'
                    type='number'
                    value={inputs.sort}
                    onChange={handleInputChange}
                  />
                </Form.Group>
                <Form.Select
                  label='状态'
                  name='status'
                  value={inputs.status}
                  options={[
                    { key: 1, text: '上架', value: 1 },
                    { key: 2, text: '下架', value: 2 },
                  ]}
                  onChange={handleInputChange}
                />
                <Button primary onClick={submitPackage}>
                  {inputs.id ? '保存套餐' : '创建套餐'}
                </Button>
                {inputs.id ? (
                  <Button basic onClick={() => setInputs(emptyPackage)}>
                    取消编辑
                  </Button>
                ) : null}
              </Form>
            </Card.Content>
          </Card>

          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h3'>支付配置</Header>
              </Card.Header>
              {['alipay', 'wechat'].map((provider) => {
                const status = paymentStatus[provider] || {};
                return (
                  <Message key={provider} positive={status.enabled} warning={!status.enabled}>
                    <Message.Header>
                      {provider === 'alipay' ? '支付宝' : '微信支付'}
                    </Message.Header>
                    {status.enabled ? (
                      <p>配置完整</p>
                    ) : (
                      <p>缺失：{(status.missing || []).join(', ')}</p>
                    )}
                  </Message>
                );
              })}
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column width={11}>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h3'>已配置套餐</Header>
              </Card.Header>
              <Table compact celled>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell>ID</Table.HeaderCell>
                    <Table.HeaderCell>名称</Table.HeaderCell>
                    <Table.HeaderCell>价格</Table.HeaderCell>
                    <Table.HeaderCell>额度</Table.HeaderCell>
                    <Table.HeaderCell>有效期</Table.HeaderCell>
                    <Table.HeaderCell>状态</Table.HeaderCell>
                    <Table.HeaderCell>操作</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {packages.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='7'>暂无套餐</Table.Cell>
                    </Table.Row>
                  ) : (
                    packages.map((item) => (
                      <Table.Row key={item.id}>
                        <Table.Cell>{item.id}</Table.Cell>
                        <Table.Cell>{item.name}</Table.Cell>
                        <Table.Cell>{formatPrice(item.price_cents)}</Table.Cell>
                        <Table.Cell>{renderQuota(item.quota, t)}</Table.Cell>
                        <Table.Cell>
                          {item.duration_days > 0
                            ? `${item.duration_days} 天`
                            : '永久'}
                        </Table.Cell>
                        <Table.Cell>
                          <Label color={item.status === 1 ? 'green' : 'grey'}>
                            {item.status === 1 ? '上架' : '下架'}
                          </Label>
                        </Table.Cell>
                        <Table.Cell>
                          <Button
                            size='tiny'
                            onClick={() => setInputs(item)}
                          >
                            编辑
                          </Button>
                          <Button
                            size='tiny'
                            color='red'
                            onClick={() => deletePackage(item.id)}
                          >
                            删除
                          </Button>
                        </Table.Cell>
                      </Table.Row>
                    ))
                  )}
                </Table.Body>
              </Table>
            </Card.Content>
          </Card>

          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                <Header as='h3'>订单运营</Header>
              </Card.Header>
              <Form>
                <Form.Group widths='equal'>
                  <Form.Input
                    label='订单号 / 流水号'
                    name='keyword'
                    value={orderFilters.keyword}
                    onChange={handleOrderFilterChange}
                    placeholder='输入订单号或支付流水'
                  />
                  <Form.Input
                    label='用户 ID'
                    name='user_id'
                    type='number'
                    min='1'
                    value={orderFilters.user_id}
                    onChange={handleOrderFilterChange}
                    placeholder='按用户筛选'
                  />
                  <Form.Select
                    label='支付渠道'
                    name='provider'
                    value={orderFilters.provider}
                    options={[
                      { key: 'all', text: '全部渠道', value: '' },
                      { key: 'mock', text: '模拟支付', value: 'mock' },
                      { key: 'alipay', text: '支付宝', value: 'alipay' },
                      { key: 'wechat', text: '微信支付', value: 'wechat' },
                    ]}
                    onChange={handleOrderFilterChange}
                  />
                  <Form.Select
                    label='订单状态'
                    name='status'
                    value={orderFilters.status}
                    options={[
                      { key: 0, text: '全部状态', value: 0 },
                      { key: 1, text: '待支付', value: 1 },
                      { key: 2, text: '已支付', value: 2 },
                      { key: 3, text: '已关闭', value: 3 },
                      { key: 4, text: '支付失败', value: 4 },
                      { key: 5, text: '已退款', value: 5 },
                    ]}
                    onChange={handleOrderFilterChange}
                  />
                </Form.Group>
                <Button primary icon='search' content='搜索' onClick={searchOrders} />
                <Button basic icon='undo' content='重置' onClick={resetOrderFilters} />
              </Form>
              <Divider />
              <Table compact celled>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell>订单号</Table.HeaderCell>
                    <Table.HeaderCell>用户</Table.HeaderCell>
                    <Table.HeaderCell>金额</Table.HeaderCell>
                    <Table.HeaderCell>额度</Table.HeaderCell>
                    <Table.HeaderCell>渠道</Table.HeaderCell>
                    <Table.HeaderCell>状态</Table.HeaderCell>
                    <Table.HeaderCell>创建时间</Table.HeaderCell>
                    <Table.HeaderCell>操作</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {orders.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='8'>暂无订单</Table.Cell>
                    </Table.Row>
                  ) : (
                    orders.map((order) => (
                      <Table.Row key={order.id}>
                        <Table.Cell>{order.order_no}</Table.Cell>
                        <Table.Cell>{order.user_id}</Table.Cell>
                        <Table.Cell>{formatPrice(order.amount_cents)}</Table.Cell>
                        <Table.Cell>{renderQuota(order.quota, t)}</Table.Cell>
                        <Table.Cell>{order.provider}</Table.Cell>
                        <Table.Cell>
                          <Label color={orderStatusColor(order.status)}>
                            {orderStatusText(order.status)}
                          </Label>
                        </Table.Cell>
                        <Table.Cell>{formatTime(order.created_time)}</Table.Cell>
                        <Table.Cell>
                          {order.status === 1 ? (
                            <>
                              <Button
                                size='tiny'
                                basic
                                onClick={() => markPaid(order.id)}
                              >
                                补单到账
                              </Button>
                              <Button
                                size='tiny'
                                basic
                                color='grey'
                                onClick={() => closeOrder(order.id)}
                              >
                                关闭
                              </Button>
                            </>
                          ) : null}
                          {order.status === 2 ? (
                            <Button
                              size='tiny'
                              basic
                              color='orange'
                              onClick={() => refundOrder(order.id)}
                            >
                              退款作废
                            </Button>
                          ) : null}
                        </Table.Cell>
                      </Table.Row>
                    ))
                  )}
                </Table.Body>
              </Table>
              <Divider />
              <Button basic icon='refresh' content='刷新' onClick={() => loadOrders()} />
              <Button
                basic
                icon='chevron left'
                content='上一页'
                disabled={orderPage === 0}
                onClick={() => loadOrders(Math.max(orderPage - 1, 0))}
              />
              <Button
                basic
                icon='chevron right'
                content='下一页'
                disabled={orders.length < 10}
                onClick={() => loadOrders(orderPage + 1)}
              />
            </Card.Content>
          </Card>
        </Grid.Column>
      </Grid>
    </div>
  );
};

export default Package;
