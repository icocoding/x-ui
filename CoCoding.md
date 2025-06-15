# Co-coding 扩展功能

## 本地开发
> `go get -u` 更新开发包
- 启动Web服务
`go run main.go run`
- 测试

    打开网址
    > http://127.0.0.1:54321

- Macbook m1 arm64 开发

    [https://github.com/XTLS/Xray-core/releases](https://github.com/XTLS/Xray-core/releases) 下载xray-core，放在bin目录下，重命名为 `xray-darwin-amd64`

- 升级go1.22.2
- go mod tidy


## 添加功能

- 快捷配置

    > 快捷配置能够配置一些常用参数，例如：入站端口范围、入站TLS证书，以免每次添加入站节点时需要手动输入。

- 节点复制

    > 入站节点操作增加复制功能，可以快速复制节点信息，端口和ID会重新生成，主要复制TLS证书信息。

- 订阅功能

    > 添加订阅，订阅包括多个节点，可设置自动更新，防止端口被墙，可生成订阅链接，可设置有效日期


## 安装&升级

```shell
bash <(curl -Ls https://github.com/icocoding/x-ui/releases/download/tools/install-v1.0.sh)
```
