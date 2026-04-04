package codex

type Options struct {
	ExecutablePath string
	Env            map[string]string
	BaseURL        string
	APIKey         string
}

type Client struct {
	executor *executor
}

func New(options Options) *Client {
	return &Client{
		executor: newExecutor(options),
	}
}

func (c *Client) StartThread(options ThreadOptions) *Thread {
	return newThread(c.executor, options, "")
}

func (c *Client) ResumeThread(id string, options ThreadOptions) *Thread {
	return newThread(c.executor, options, id)
}
