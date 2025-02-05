package cloudflare

const (
	endpoint = "https://api.cloudflare.com/client/v4/zones"
	ID       = "CLOUDFLARE_AUTH_EMAIL"
	KEY      = "CLOUDFLARE_AUTH_KEY"
)

type Response struct {
	Comment   string `json:"comment"`
	Content   string `json:"content"`
	Name      string `json:"name"`
	Ttl       int    `json:"ttl"`
	Type      string `json:"type"`
	Id        string `json:"id"`
	Proxiable bool   `json:"proxiable"`
	Proxied   bool   `json:"proxied"`
	Settings  struct {
		Ipv4_only bool `json:"ipv4_only"`
		Ipv6_only bool `json:"ipv6_only"`
	} `json:"settings"`
	Tags []string `json:"tags"`
}

type cloudflareResponse struct {
	cloudflareStatus
	Result     []Response
	ResultInfo struct {
		Count       int `json:"count"`
		Page        int `json:"page"`
		Per_page    int `json:"per_page"`
		Total_count int `json:"total_count"`
	} `json:"result_info"`
}

type cloudflareStatus struct {
	Success  bool `json:"success"`
	Messages struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
}

type cloudflareRequest struct {
}

type cloudflare struct {
	Email string
	Key   string
}

func NewCloudflare(email, key string) *cloudflare {
	return &cloudflare{
		Email: email,
		Key:   key,
	}
}

func (c *cloudflare) ListRecords(domain string, response *cloudflareResponse) error {
	return nil
}

func (c *cloudflare) CreateRecord(domain, subDomain, value string, status *cloudflareStatus) error {
	return nil
}

func (c *cloudflare) ModfiyRecord(domain string, recordId int, subDomain, recordLine, value string, status *cloudflareStatus) error {
	return nil
}

func (c *cloudflare) DeleteRecord(Domain string, RecordId int, status *cloudflareStatus) error {
	return nil
}

func (c *cloudflare) request(service, action, version string, params, result any) error {

	return nil
}
