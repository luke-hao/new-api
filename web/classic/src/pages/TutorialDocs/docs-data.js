const API_BASE_URL = 'https://code28.ccwu.cc/v1';
const DOCS_URL = 'https://code28.ccwu.cc/tutorial-docs';

export const docsNavigation = [
  {
    title: '入门',
    items: [{ path: 'intro', title: '可乐AI 接入总览' }],
  },
  {
    title: '可乐AI 使用说明',
    items: [
      { path: 'kola/register-and-recharge', title: '注册与充值' },
      { path: 'kola/api-key', title: 'API Key 的创建与维护' },
      { path: 'kola/quota-and-usage', title: '额度与使用记录' },
    ],
  },
  {
    title: 'Codex 桌面端',
    items: [
      { path: 'codex-desktop/download', title: 'Codex 桌面端下载' },
      { path: 'codex-desktop/cc-switch', title: 'Codex 配置与 CC Switch' },
      { path: 'codex-desktop/usage', title: 'Codex 的使用说明' },
    ],
  },
  {
    title: 'Codex CLI',
    items: [
      { path: 'codex-cli/install', title: 'Codex CLI 安装' },
      { path: 'codex-cli/configuration', title: 'Codex CLI 配置可乐AI' },
      { path: 'codex-cli/usage', title: 'Codex CLI 使用说明' },
    ],
  },
  {
    title: '聊天客户端',
    items: [
      { path: 'chat-clients/cherry-studio', title: 'Cherry Studio 接入可乐AI' },
      {
        path: 'chat-clients/openai-compatible',
        title: '通用 OpenAI Compatible 聊天客户端',
      },
    ],
  },
  {
    title: '编辑器插件',
    items: [
      { path: 'editor-plugins/overview', title: '编辑器插件总览' },
      { path: 'editor-plugins/vscode', title: 'VS Code 插件配置' },
      {
        path: 'editor-plugins/cursor-and-others',
        title: 'Cursor 与其它编辑器',
      },
    ],
  },
  {
    title: 'Agent',
    items: [
      { path: 'agents/openclaw', title: 'OpenClaw 接入可乐AI' },
      { path: 'agents/hermes', title: 'Hermes 接入可乐AI' },
    ],
  },
  {
    title: '排错',
    items: [{ path: 'troubleshooting/common', title: '常见排错' }],
  },
];

export const flatDocs = docsNavigation.flatMap((group) => group.items);

export const docsByPath = {
  intro: {
    description:
      '把可乐AI 的 API Key、Base URL 和常见客户端配置放在一套站内教程里，适合第一次接入 New API 的用户按顺序完成配置。',
    sections: [
      {
        id: 'what-is-kola',
        title: '先认识可乐AI 与 New API',
        content: `可乐AI 是你的统一 AI API 入口。New API 负责把不同上游模型整理成兼容 OpenAI 风格的接口，你只需要在客户端里配置统一的 Base URL 和 API Key，就可以开始调用。

最常用的两个信息是：

- 文档地址：\`${DOCS_URL}\`
- API Base URL：\`${API_BASE_URL}\`

如果一个客户端要求填写完整接口地址，通常是在 Base URL 后继续拼接对应端点，例如 \`${API_BASE_URL}/chat/completions\`。`,
      },
      {
        id: 'basic-flow',
        title: '推荐接入流程',
        content: `建议按下面顺序完成：

1. 注册并登录可乐AI。
2. 根据需要完成充值或获得可用额度。
3. 创建一个新的 API Key。
4. 在客户端里填写 \`${API_BASE_URL}\`。
5. 把 API Key 填入客户端的 Key、Token 或 Authorization 字段。
6. 选择一个可用模型，发送一次简单测试请求。

多数问题都出在 Base URL 少了 \`/v1\`、API Key 复制不完整、模型名不在当前分组可用范围内。`,
      },
      {
        id: 'quick-test',
        title: '快速测试',
        content: `创建 API Key 后，可以用兼容 OpenAI 的方式测试：

\`\`\`bash
curl ${API_BASE_URL}/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      { "role": "user", "content": "你好，请用一句话介绍可乐AI。" }
    ]
  }'
\`\`\`

如果返回模型回复，说明 Key、额度、模型和接口地址都已经连通。`,
      },
    ],
  },
  'kola/register-and-recharge': {
    description:
      '完成账号注册、登录和充值后，账户才会拥有可用于模型调用的额度。',
    sections: [
      {
        id: 'register',
        title: '注册账号',
        content: `打开可乐AI 站点后，点击注册并按页面提示完成账号创建。注册完成后建议先进入个人中心确认邮箱、用户名和登录方式是否正确。

如果站点开启了邀请码、邮箱验证或人机验证，请按页面提示完成。不同站点配置可能略有不同，以当前页面显示为准。`,
      },
      {
        id: 'recharge',
        title: '充值与到账',
        content: `进入控制台后，打开充值或钱包页面，选择适合的充值方式并完成支付。

充值完成后，回到账户余额页面确认额度是否到账。若支付成功但余额未更新，可以先刷新页面，再检查支付订单状态。`,
      },
      {
        id: 'after-recharge',
        title: '充值后做什么',
        content: `充值不是最后一步。为了让客户端可以调用模型，还需要：

- 创建 API Key。
- 确认可用分组和模型列表。
- 在客户端填写 \`${API_BASE_URL}\`。
- 发送一次测试请求，确认额度可以正常消耗。`,
      },
    ],
  },
  'kola/api-key': {
    description:
      'API Key 是客户端访问可乐AI 的凭证，请按用途创建、保存和轮换。',
    sections: [
      {
        id: 'create-key',
        title: '创建 API Key',
        content: `登录可乐AI 控制台后，进入令牌或 API Key 页面，点击创建。

建议填写容易识别的名称，例如：

- \`codex-desktop-main\`
- \`codex-cli-laptop\`
- \`cherry-studio-home\`
- \`vscode-plugin-work\`

如果页面提供分组、额度、过期时间或速率限制选项，请按实际用途设置。`,
      },
      {
        id: 'save-key',
        title: '保存与使用',
        content: `API Key 通常只完整显示一次。创建后请立刻保存到密码管理器或本机安全笔记里。

客户端常见填写方式：

- API Key：填入创建得到的完整 Key。
- Authorization：填入 \`Bearer YOUR_API_KEY\`。
- Base URL：填入 \`${API_BASE_URL}\`。

不要把 API Key 发到公开聊天群、截图、日志、公开仓库或交给不可信软件。`,
      },
      {
        id: 'rotate-key',
        title: '维护与轮换',
        content: `建议为不同客户端使用不同 Key。这样某个客户端不再使用时，只需要禁用对应 Key，不会影响其它工具。

出现下面情况时，应立即禁用旧 Key 并重新创建：

- Key 被发到公开环境。
- 客户端设备丢失或转交他人。
- 调用记录出现异常消耗。
- 准备更换长期使用的工具或工作流。`,
      },
    ],
  },
  'kola/quota-and-usage': {
    description:
      '通过余额、日志和模型价格信息判断每次调用是否正常，以及额度消耗是否符合预期。',
    sections: [
      {
        id: 'quota',
        title: '额度是什么',
        content: `额度用于支付模型调用产生的费用。不同模型、不同输入输出长度、不同端点类型可能有不同消耗。

如果你刚开始使用，建议先选择成本较低的模型测试，再切换到更强模型处理正式任务。`,
      },
      {
        id: 'usage-log',
        title: '查看使用记录',
        content: `在控制台的日志或使用记录页面，可以查看调用时间、模型、状态、消耗和失败原因。

排查问题时，优先看三类信息：

- 请求是否到达可乐AI。
- 模型名称是否可用。
- 失败原因是否提示余额、权限、分组或上游错误。`,
      },
      {
        id: 'control-cost',
        title: '控制消耗',
        content: `想让额度更耐用，可以从这些地方入手：

- 给测试 Key 设置较低额度。
- 避免把超长文件一次性塞进对话。
- 对批量任务先小样本测试。
- 定期查看高消耗模型和异常失败请求。
- 只给自动化工具开放必要模型。`,
      },
    ],
  },
  'codex-desktop/download': {
    description:
      'Codex 桌面端适合需要在本机项目、文件和任务之间连续工作的用户。',
    sections: [
      {
        id: 'download',
        title: '下载方式',
        content: `请优先从 OpenAI 官方页面或可信来源下载 Codex 桌面端，并按系统提示完成安装。

安装完成后，先打开应用确认能正常启动，再进行可乐AI 的接口配置。`,
      },
      {
        id: 'before-config',
        title: '配置前准备',
        content: `开始配置前准备好两项信息：

- API Base URL：\`${API_BASE_URL}\`
- API Key：在可乐AI 控制台创建的新 Key

如果你同时使用多个 AI 服务，建议给可乐AI 单独建一个配置项，名称可以写成 \`Kola AI\` 或 \`可乐AI\`。`,
      },
      {
        id: 'safety',
        title: '安全建议',
        content: `桌面端通常能访问本机文件和项目，因此更要注意 Key 的保存方式。

- 不要把 Key 写进项目源码。
- 不要把配置文件上传到公开仓库。
- 多人共用设备时，建议使用单独系统账户。
- 长时间不用的 Key 可以禁用。`,
      },
    ],
  },
  'codex-desktop/cc-switch': {
    description:
      'CC Switch 适合在多个服务配置之间快速切换，避免反复手动改 Base URL 和 Key。',
    sections: [
      {
        id: 'purpose',
        title: '什么时候需要 CC Switch',
        content: `如果你经常在官方服务、可乐AI、测试环境或多个账号之间切换，可以使用 CC Switch 管理配置。

它的价值是把不同服务保存成不同 Profile，使用时选择对应 Profile 即可。`,
      },
      {
        id: 'profile',
        title: '可乐AI Profile 建议',
        content: `建议新建一个可乐AI Profile：

- 名称：\`Kola AI\`
- Base URL：\`${API_BASE_URL}\`
- API Key：填入可乐AI 控制台创建的 Key
- 默认模型：选择当前分组中可用的模型

保存后切换到该 Profile，再回到 Codex 发起一次简单任务测试。`,
      },
      {
        id: 'check',
        title: '切换后检查',
        content: `切换 Profile 后，如果请求失败，先检查：

- 当前 Profile 是否真的选中。
- Base URL 是否包含 \`/v1\`。
- Key 是否属于可乐AI。
- 模型名称是否在可乐AI 当前分组可用。`,
      },
    ],
  },
  'codex-desktop/usage': {
    description: '用 Codex 处理真实任务时，先让它理解目标和边界，再逐步执行。',
    sections: [
      {
        id: 'good-prompt',
        title: '写清楚任务',
        content: `给 Codex 的任务越清楚，执行越稳定。推荐包含：

- 你想完成什么。
- 哪些文件或目录相关。
- 哪些行为不能做。
- 完成后要如何验证。

例如：“阅读这个项目的登录流程，找出注册失败的原因，只改必要文件，最后跑相关测试。”`,
      },
      {
        id: 'iterate',
        title: '分步推进',
        content: `复杂任务不要一次性要求它完成所有事情。可以先让 Codex 读代码并说明方案，再确认实现，再跑测试。

这样更容易发现误解，也能减少无关改动。`,
      },
      {
        id: 'review',
        title: '完成后复核',
        content: `任务完成后建议看三件事：

- 修改是否只落在相关文件。
- 测试或构建是否通过。
- 是否有密钥、隐私信息、临时日志被写入代码。`,
      },
    ],
  },
  'codex-cli/install': {
    description: 'Codex CLI 适合习惯终端、脚本和仓库工作流的用户。',
    sections: [
      {
        id: 'install',
        title: '安装前确认',
        content: `安装 Codex CLI 前，先确认本机已经具备基础命令行环境，并能正常访问可乐AI 站点。

安装命令和包名可能随官方版本变化，请以官方说明为准。安装完成后，在终端运行版本检查命令确认 CLI 可用。`,
      },
      {
        id: 'prepare',
        title: '准备配置',
        content: `准备以下信息：

- API Base URL：\`${API_BASE_URL}\`
- API Key：在可乐AI 控制台创建
- 默认模型：使用可乐AI 模型广场或控制台里可见的模型名

如果 CLI 支持环境变量和配置文件两种方式，建议个人设备使用配置文件，临时脚本使用环境变量。`,
      },
      {
        id: 'verify',
        title: '安装后验证',
        content: `配置完成后，在一个测试目录里发起简单任务，例如让 Codex 总结 README 或解释一个小脚本。

如果终端能收到模型回复，说明 CLI 与可乐AI 的链路已经打通。`,
      },
    ],
  },
  'codex-cli/configuration': {
    description:
      '把 Codex CLI 的接口地址指向可乐AI，并使用可乐AI 创建的 API Key。',
    sections: [
      {
        id: 'base-url',
        title: 'Base URL',
        content: `Codex CLI 中的 Base URL 填：

\`\`\`text
${API_BASE_URL}
\`\`\`

注意末尾不要写成站点首页，也不要漏掉 \`/v1\`。`,
      },
      {
        id: 'key',
        title: 'API Key',
        content: `API Key 使用可乐AI 控制台创建的 Key。

如果 CLI 使用环境变量，常见形式类似：

\`\`\`bash
export OPENAI_API_KEY="YOUR_API_KEY"
export OPENAI_BASE_URL="${API_BASE_URL}"
\`\`\`

如果 CLI 使用配置文件，就在对应字段里填写同样的 Key 和 Base URL。`,
      },
      {
        id: 'model',
        title: '模型名称',
        content: `模型名称必须使用可乐AI 当前账号可见、当前分组可用的模型。

不确定模型名时，先到模型广场或控制台复制模型名称，再粘贴到 Codex CLI 的默认模型配置中。`,
      },
    ],
  },
  'codex-cli/usage': {
    description: '在终端中使用 Codex CLI 时，建议从小任务开始，逐步扩大范围。',
    sections: [
      {
        id: 'small-task',
        title: '从小任务开始',
        content: `首次使用建议选择低风险任务：

- 总结某个文件。
- 解释一段报错。
- 列出可能相关的代码位置。
- 生成不直接写入文件的方案。

确认输出质量和速度后，再让它执行实际改动。`,
      },
      {
        id: 'workspace',
        title: '在项目里使用',
        content: `进入项目目录后再启动 Codex CLI，可以让它更容易读取上下文。

如果任务涉及敏感文件、生产配置或大范围改动，请在指令里明确边界。`,
      },
      {
        id: 'troubleshoot',
        title: '调用失败时',
        content: `失败时优先检查：

- Key 是否正确。
- Base URL 是否是 \`${API_BASE_URL}\`。
- 当前模型是否可用。
- 账户余额是否充足。
- 控制台日志中是否有更明确的错误信息。`,
      },
    ],
  },
  'chat-clients/cherry-studio': {
    description:
      'Cherry Studio 支持 OpenAI Compatible 配置，可以把可乐AI 作为自定义服务接入。',
    sections: [
      {
        id: 'add-provider',
        title: '新增服务商',
        content: `在 Cherry Studio 中新增 OpenAI Compatible 或自定义 OpenAI 服务商。

填写建议：

- 名称：可乐AI
- API Key：可乐AI 控制台创建的 Key
- API 地址或 Base URL：\`${API_BASE_URL}\`
- 模型：填入可乐AI 可用模型名称`,
      },
      {
        id: 'model-test',
        title: '添加模型并测试',
        content: `保存服务商后，添加一个轻量模型进行测试。

测试消息可以写：“请用一句话说明当前连接是否正常。”如果能返回回复，再添加其它常用模型。`,
      },
      {
        id: 'common-fields',
        title: '字段差异',
        content: `不同版本客户端字段名可能不同：

- API Host、API Endpoint、Base URL 通常都填 \`${API_BASE_URL}\`。
- Token、Secret Key、API Key 通常都填可乐AI Key。
- 如果客户端要求完整路径，再使用 \`${API_BASE_URL}/chat/completions\`。`,
      },
    ],
  },
  'chat-clients/openai-compatible': {
    description:
      '凡是支持 OpenAI Compatible 的客户端，大多可以用同一组地址和 Key 接入可乐AI。',
    sections: [
      {
        id: 'standard-config',
        title: '通用配置',
        content: `通用填写方式：

| 字段 | 填写 |
| --- | --- |
| Base URL | \`${API_BASE_URL}\` |
| API Key | 可乐AI 控制台创建的 Key |
| Authorization | \`Bearer YOUR_API_KEY\` |
| Chat Endpoint | \`/chat/completions\` |

如果客户端已经会自动拼接 \`/chat/completions\`，Base URL 不要重复写完整端点。`,
      },
      {
        id: 'model-list',
        title: '模型列表',
        content: `有些客户端可以自动拉取模型列表，有些需要手动填写模型名称。

自动拉取失败时，可以在可乐AI 控制台复制模型名称后手动添加。`,
      },
      {
        id: 'compatibility',
        title: '兼容性提醒',
        content: `OpenAI Compatible 主要表示接口格式接近 OpenAI 风格，但不同客户端对图片、音频、工具调用、流式输出的支持程度可能不同。

遇到高级功能异常时，先用普通文本对话测试基础链路，再逐项开启高级能力。`,
      },
    ],
  },
  'editor-plugins/overview': {
    description:
      '编辑器插件适合把可乐AI 接入日常编码、补全、解释和重构工作流。',
    sections: [
      {
        id: 'choose-plugin',
        title: '选择插件',
        content: `优先选择明确支持 OpenAI Compatible、自定义 Base URL、自定义 API Key 的插件。

如果插件只能绑定固定服务商，通常不适合接入可乐AI。`,
      },
      {
        id: 'shared-config',
        title: '通用配置',
        content: `多数插件需要三类信息：

- Base URL：\`${API_BASE_URL}\`
- API Key：可乐AI 控制台创建的 Key
- Model：当前分组可用模型名称

配置完成后，先让插件解释当前文件的一小段代码，确认可用后再使用自动改写功能。`,
      },
      {
        id: 'scope',
        title: '控制权限范围',
        content: `编辑器插件可能读取当前文件、打开的工作区或终端上下文。

使用前请确认插件权限，避免把敏感配置、私钥、生产数据作为上下文发送。`,
      },
    ],
  },
  'editor-plugins/vscode': {
    description:
      'VS Code 插件接入可乐AI 时，核心仍是自定义 Base URL、API Key 和模型名。',
    sections: [
      {
        id: 'install-plugin',
        title: '安装插件',
        content: `在 VS Code 扩展市场中选择支持 OpenAI Compatible 的 AI 插件并安装。

安装后进入插件设置页面，寻找 Provider、Base URL、API Key、Model 等字段。`,
      },
      {
        id: 'configure-plugin',
        title: '填写可乐AI 配置',
        content: `推荐配置：

- Provider：OpenAI Compatible 或 Custom OpenAI
- Base URL：\`${API_BASE_URL}\`
- API Key：可乐AI 控制台创建的 Key
- Model：可乐AI 可用模型

保存后重新加载窗口或重启插件，让配置生效。`,
      },
      {
        id: 'first-test',
        title: '首次测试',
        content: `打开一个非敏感测试文件，让插件完成解释、注释或生成单元测试建议。

如果失败，回到可乐AI 控制台查看请求日志，通常能看到模型名、额度或鉴权问题。`,
      },
    ],
  },
  'editor-plugins/cursor-and-others': {
    description:
      'Cursor 与其它编辑器能否接入，取决于它们是否开放自定义模型服务配置。',
    sections: [
      {
        id: 'check-support',
        title: '先看是否支持自定义接口',
        content: `查看编辑器或插件是否支持：

- OpenAI Compatible
- Custom API Base URL
- 自定义 API Key
- 自定义模型名称

如果这些字段都没有，通常无法直接使用可乐AI。`,
      },
      {
        id: 'fill-config',
        title: '填写方式',
        content: `如果支持自定义接口，按下面填写：

- Base URL：\`${API_BASE_URL}\`
- Key：可乐AI API Key
- Model：可乐AI 可用模型

保存后先用小文件测试，不要一开始就对整个项目执行自动改写。`,
      },
      {
        id: 'fallback',
        title: '替代方案',
        content: `如果编辑器本体不支持自定义接口，可以考虑：

- 使用支持自定义接口的扩展。
- 使用 Codex CLI 在项目目录里工作。
- 使用 Cherry Studio 处理问答和代码片段。`,
      },
    ],
  },
  'agents/openclaw': {
    description:
      'OpenClaw 类 Agent 接入可乐AI 时，通常需要配置 OpenAI 兼容接口。',
    sections: [
      {
        id: 'agent-config',
        title: '基础配置',
        content: `在 Agent 的模型服务配置中选择 OpenAI Compatible 或自定义 OpenAI。

填写：

- Base URL：\`${API_BASE_URL}\`
- API Key：可乐AI 控制台创建的 Key
- Model：当前任务需要的可用模型`,
      },
      {
        id: 'permissions',
        title: '权限与工具',
        content: `Agent 往往可以调用工具、读写文件或访问网络。建议先关闭不必要工具，只开放当前任务需要的能力。

第一次接入时，先让 Agent 完成只读任务，确认行为稳定后再允许写入或执行命令。`,
      },
      {
        id: 'logs',
        title: '观察日志',
        content: `如果 Agent 连续失败，检查两边日志：

- Agent 本地日志：看请求地址、模型名和返回错误。
- 可乐AI 控制台日志：看鉴权、额度、模型分组和上游状态。`,
      },
    ],
  },
  'agents/hermes': {
    description: 'Hermes 类 Agent 接入方式与其它 OpenAI Compatible 工具类似。',
    sections: [
      {
        id: 'base',
        title: '接口配置',
        content: `在 Hermes 的模型配置里填写：

\`\`\`text
Base URL: ${API_BASE_URL}
API Key: YOUR_API_KEY
Model: 选择可乐AI 可用模型
\`\`\`

如果配置项区分 Chat、Embedding 或其它端点，请确保模型类型和端点能力匹配。`,
      },
      {
        id: 'task-size',
        title: '控制任务规模',
        content: `Agent 任务越大，消耗和失败概率越高。建议先把目标拆小：

- 先读取和总结。
- 再提出计划。
- 最后执行具体动作。

这样便于观察每一步是否符合预期。`,
      },
      {
        id: 'recover',
        title: '失败恢复',
        content: `遇到失败时，不要连续重试同一个大任务。先缩小上下文，换一个轻量测试问题确认连接，再回到原任务。`,
      },
    ],
  },
  'troubleshooting/common': {
    description:
      '大部分接入问题都可以从地址、Key、模型、额度和日志五个方向快速定位。',
    sections: [
      {
        id: 'checklist',
        title: '五步检查',
        content: `遇到调用失败，按这个顺序检查：

1. Base URL 是否为 \`${API_BASE_URL}\`。
2. API Key 是否复制完整、没有多余空格。
3. 模型名称是否可用。
4. 账户余额或 Key 额度是否充足。
5. 可乐AI 控制台日志是否有明确失败原因。`,
      },
      {
        id: 'errors',
        title: '常见现象',
        content: `常见现象与方向：

- 401 或鉴权失败：检查 Key。
- 404 或路径不存在：检查 Base URL 与端点拼接。
- 模型不存在：检查模型名和分组。
- 余额不足：充值或更换额度充足的 Key。
- 请求超时：降低上下文长度，或稍后重试。`,
      },
      {
        id: 'ask-support',
        title: '联系支持前准备',
        content: `如果需要联系站点支持，请准备：

- 发生时间。
- 使用的客户端名称。
- 模型名称。
- 可乐AI 控制台里的请求日志截图或错误信息。
- 不要发送完整 API Key。`,
      },
    ],
  },
};

export { API_BASE_URL, DOCS_URL };
