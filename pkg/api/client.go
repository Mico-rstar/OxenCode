package api

// Client API 客户端
type Client struct {
	// TODO: 添加 HTTP 客户端和配置
}

// NewClient 创建新的 API 客户端
func NewClient() *Client {
	return &Client{}
}

// Chat 发送聊天请求
func (c *Client) Chat(message string) (string, error) {
	// TODO: 实现 API 调用
	return "", nil
}
