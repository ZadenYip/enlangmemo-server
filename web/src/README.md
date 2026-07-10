# 说明

## proxy.conf.json

proxy.conf.json 是给 Angular 开发服务器用的代理配置文件，用来把开发服务器的请求来源从 4200 端口映射到 8080 端口。
避免浏览器因为不 [同源](https://developer.mozilla.org/zh-CN/docs/Web/Security/Defenses/Same-origin_policy)，而服务器也没显示允许跨源从而阻止 JS 脚本接受 HTTP 响应结果。