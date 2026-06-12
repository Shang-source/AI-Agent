import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Button,
  Card,
  Grid,
  Header,
  Icon,
  Label,
  List,
  Message,
  Segment,
  Statistic,
  Table,
} from 'semantic-ui-react';
import { API, copy, showError, showSuccess, showWarning } from '../../helpers';
import { renderQuota } from '../../helpers/render';
import { useTranslation } from 'react-i18next';
import './index.css';

const UsageGuide = () => {
  const { t } = useTranslation();
  const [serverAddress, setServerAddress] = useState(window.location.origin);
  const [tokens, setTokens] = useState([]);
  const [models, setModels] = useState([]);
  const [userQuota, setUserQuota] = useState(0);
  const [loading, setLoading] = useState(true);

  const activeToken = useMemo(() => {
    return tokens.find((token) => token.status === 1) || tokens[0];
  }, [tokens]);

  const apiKey = activeToken ? `sk-${activeToken.key}` : 'sk-your-api-key';
  const baseUrl = `${serverAddress.replace(/\/$/, '')}/v1`;
  const modelName = models[0] || '请替换为可用模型名';

  const loadServerAddress = () => {
    const status = localStorage.getItem('status');
    if (!status) return;
    try {
      const data = JSON.parse(status);
      if (data.server_address) {
        setServerAddress(data.server_address);
      }
    } catch (err) {
      // Ignore malformed local status and use current origin.
    }
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [tokenRes, modelRes, userRes] = await Promise.all([
        API.get('/api/token/?p=0'),
        API.get('/api/models'),
        API.get('/api/user/self'),
      ]);

      if (tokenRes.data.success) {
        setTokens(tokenRes.data.data || []);
      } else {
        showError(tokenRes.data.message);
      }

      if (modelRes.data.success) {
        const modelData = modelRes.data.data || {};
        const modelNames = Array.isArray(modelData)
          ? modelData
          : Object.values(modelData).flat();
        setModels([...new Set(modelNames)].filter(Boolean));
      }

      if (userRes.data.success) {
        setUserQuota(userRes.data.data.quota || 0);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const copyText = async (text, label) => {
    if (await copy(text)) {
      showSuccess(`${label}已复制`);
    } else {
      showWarning('复制失败，请手动选择文本复制');
    }
  };

  useEffect(() => {
    loadServerAddress();
    loadData().then();
  }, []);

  const curlExample = `curl ${baseUrl}/chat/completions \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${modelName}",
    "messages": [
      {"role": "user", "content": "你好，帮我写一句产品介绍"}
    ]
  }'`;

  const nodeExample = `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "${apiKey}",
  baseURL: "${baseUrl}"
});

const completion = await client.chat.completions.create({
  model: "${modelName}",
  messages: [{ role: "user", content: "你好" }]
});

console.log(completion.choices[0].message.content);`;

  const pythonExample = `from openai import OpenAI

client = OpenAI(
    api_key="${apiKey}",
    base_url="${baseUrl}"
)

completion = client.chat.completions.create(
    model="${modelName}",
    messages=[{"role": "user", "content": "你好"}]
)

print(completion.choices[0].message.content)`;

  const difyConfig = `Model Provider: OpenAI-API-compatible
API Key: ${apiKey}
API Base URL: ${baseUrl}
Model Name: ${modelName}`;

  const chatboxConfig = `API Host: ${baseUrl}
API Key: ${apiKey}
Model: ${modelName}`;

  const CodeBlock = ({ title, description, code }) => (
    <Segment className='usage-code-block'>
      <div className='usage-code-header'>
        <div>
          <Header as='h4'>{title}</Header>
          <p>{description}</p>
        </div>
        <Button
          basic
          icon='copy'
          content='复制'
          onClick={() => copyText(code, title)}
        />
      </div>
      <pre>
        <code>{code}</code>
      </pre>
    </Segment>
  );

  return (
    <div className='dashboard-container usage-guide'>
      <Grid stackable>
        <Grid.Row columns={3}>
          <Grid.Column>
            <Card fluid className='chart-card usage-stat-card'>
              <Card.Content>
                <Statistic size='small'>
                  <Statistic.Label>Base URL</Statistic.Label>
                  <Statistic.Value className='usage-stat-text'>
                    {baseUrl}
                  </Statistic.Value>
                </Statistic>
                <Button
                  basic
                  fluid
                  icon='copy'
                  content='复制 Base URL'
                  onClick={() => copyText(baseUrl, 'Base URL')}
                />
              </Card.Content>
            </Card>
          </Grid.Column>
          <Grid.Column>
            <Card fluid className='chart-card usage-stat-card'>
              <Card.Content>
                <Statistic size='small'>
                  <Statistic.Label>当前额度</Statistic.Label>
                  <Statistic.Value className='usage-stat-text'>
                    {renderQuota(userQuota, t)}
                  </Statistic.Value>
                </Statistic>
                <Button
                  as={Link}
                  to='/topup'
                  primary
                  fluid
                  icon='cart'
                  content='去充值'
                />
              </Card.Content>
            </Card>
          </Grid.Column>
          <Grid.Column>
            <Card fluid className='chart-card usage-stat-card'>
              <Card.Content>
                <Statistic size='small'>
                  <Statistic.Label>示例令牌</Statistic.Label>
                  <Statistic.Value className='usage-stat-text'>
                    {activeToken ? activeToken.name || `#${activeToken.id}` : '未创建'}
                  </Statistic.Value>
                </Statistic>
                <Button
                  as={Link}
                  to='/token'
                  color={activeToken ? undefined : 'orange'}
                  fluid
                  icon='key'
                  content={activeToken ? '管理令牌' : '创建令牌'}
                />
              </Card.Content>
            </Card>
          </Grid.Column>
        </Grid.Row>

        <Grid.Row>
          <Grid.Column width={6}>
            <Card fluid className='chart-card'>
              <Card.Content>
                <Card.Header>
                  <Header as='h2'>API 接入指南</Header>
                </Card.Header>
                <List divided relaxed className='usage-steps'>
                  <List.Item>
                    <Icon name='cart' />
                    <List.Content>
                      <List.Header>充值额度</List.Header>
                      <List.Description>
                        用户余额不足时先到充值中心购买套餐，额度到账后即可调用 API。
                      </List.Description>
                    </List.Content>
                  </List.Item>
                  <List.Item>
                    <Icon name='key' />
                    <List.Content>
                      <List.Header>创建令牌</List.Header>
                      <List.Description>
                        每个应用或客户建议使用单独令牌，便于限额、停用和排查日志。
                      </List.Description>
                    </List.Content>
                  </List.Item>
                  <List.Item>
                    <Icon name='plug' />
                    <List.Content>
                      <List.Header>填入 OpenAI 兼容配置</List.Header>
                      <List.Description>
                        大多数工具只需要 Base URL、API Key 和模型名三项。
                      </List.Description>
                    </List.Content>
                  </List.Item>
                  <List.Item>
                    <Icon name='chart line' />
                    <List.Content>
                      <List.Header>查看消耗</List.Header>
                      <List.Description>
                        令牌页看单个 Key 消耗，日志页看每次请求的模型和额度。
                      </List.Description>
                    </List.Content>
                  </List.Item>
                </List>
                {!activeToken ? (
                  <Message warning>
                    <Message.Header>还没有可用令牌</Message.Header>
                    <p>先创建令牌，再把生成的 Key 填入客户端或 SDK。</p>
                  </Message>
                ) : null}
              </Card.Content>
            </Card>
          </Grid.Column>

          <Grid.Column width={10}>
            <Card fluid className='chart-card'>
              <Card.Content>
                <Card.Header>
                  <Header as='h3'>客户端配置</Header>
                </Card.Header>
                <Table compact celled>
                  <Table.Body>
                    <Table.Row>
                      <Table.Cell width={4}>API Key</Table.Cell>
                      <Table.Cell>
                        <code>{apiKey}</code>
                      </Table.Cell>
                      <Table.Cell width={3}>
                        <Button
                          basic
                          size='tiny'
                          icon='copy'
                          content='复制'
                          onClick={() => copyText(apiKey, 'API Key')}
                        />
                      </Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>Base URL</Table.Cell>
                      <Table.Cell>
                        <code>{baseUrl}</code>
                      </Table.Cell>
                      <Table.Cell>
                        <Button
                          basic
                          size='tiny'
                          icon='copy'
                          content='复制'
                          onClick={() => copyText(baseUrl, 'Base URL')}
                        />
                      </Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>请求地址</Table.Cell>
                      <Table.Cell>
                        <code>{baseUrl}/chat/completions</code>
                      </Table.Cell>
                      <Table.Cell>
                        <Button
                          basic
                          size='tiny'
                          icon='copy'
                          content='复制'
                          onClick={() =>
                            copyText(`${baseUrl}/chat/completions`, '请求地址')
                          }
                        />
                      </Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>模型名</Table.Cell>
                      <Table.Cell>
                        {models.length > 0 ? (
                          <Label color='blue'>{modelName}</Label>
                        ) : (
                          <Label color='grey'>按后台开放模型填写</Label>
                        )}
                      </Table.Cell>
                      <Table.Cell>
                        <Button
                          basic
                          size='tiny'
                          icon='refresh'
                          content='刷新'
                          loading={loading}
                          onClick={loadData}
                        />
                      </Table.Cell>
                    </Table.Row>
                  </Table.Body>
                </Table>
              </Card.Content>
            </Card>
          </Grid.Column>
        </Grid.Row>

        <Grid.Row columns={2}>
          <Grid.Column>
            <CodeBlock
              title='curl'
              description='适合快速测试账号、额度和渠道是否可用。'
              code={curlExample}
            />
          </Grid.Column>
          <Grid.Column>
            <CodeBlock
              title='Node.js SDK'
              description='适合接入网站、自动化脚本和后端服务。'
              code={nodeExample}
            />
          </Grid.Column>
        </Grid.Row>

        <Grid.Row columns={2}>
          <Grid.Column>
            <CodeBlock
              title='Python SDK'
              description='适合数据处理、Agent 和内部工具。'
              code={pythonExample}
            />
          </Grid.Column>
          <Grid.Column>
            <CodeBlock
              title='Dify / Chatbox'
              description='把下面几项填到 OpenAI 兼容供应商配置里。'
              code={`${difyConfig}\n\nChatbox:\n${chatboxConfig}`}
            />
          </Grid.Column>
        </Grid.Row>
      </Grid>
    </div>
  );
};

export default UsageGuide;
