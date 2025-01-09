package version

import "sync"

type SystemInfo struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	Release   string `json:"release"`
	Version   string `json:"version"`
	Machine   string `json:"machine"`
	Domain    string `json:"domain,omitempty"`
	OS        string `json:"os"`
	Processor string `json:"processor"`
}

func Uname() (*SystemInfo, error) {
	return sync.OnceValues(GetSystemInfo)()
}
