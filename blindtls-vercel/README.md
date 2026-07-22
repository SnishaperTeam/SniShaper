# BlindTLS Vercel Serverless 部署端

这是经过专门适配的 BlindTLS 服务端代为握手与凭证提取 API，适用于直接托管至 **Vercel** 平台。

## 部署步骤

1\. 安装 Vercel CLI

若您已安装 Node.js 运行环境，可使用 npm 全局安装 Vercel 工具：

```
npm install -g vercel
```

*（或者直接使用 `npx vercel` 免安装运行）*

### 2\. 执行部署

进入本项目的根目录中，打开终端运行：

```
vercel
```

-   终端会提示您登录 Vercel 并进行项目初始化关联，一路回车即可。
-   初始化完成后，Vercel 会提供一个 **Preview (预览)** 域名。

若要将其发布为正式的 **Production (生产)** 线上版本：

```
vercel --prod
```

-   部署成功后，您会获得一个以 `https://your-project-name.vercel.app` 结尾的合法 HTTPS API 地址。

* * *

## 客户端配置

在获取到您的 Vercel 部署域名（例如 `https://blindtls-api.vercel.app`）后，在您本地的 Windows 客户端即可如此一键启动运行直连模式：

.\\blindtls-client.exe -mode test -api-url https://example.com/api/v1/session -cover-sni **microsoft.com** -target onedrive.live.com:443