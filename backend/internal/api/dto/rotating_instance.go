package dto

type RotatingProxyInstance struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	PortStart  int    `json:"port_start"`
	PortEnd    int    `json:"port_end"`
	UsedPorts  int    `json:"used_ports"`
	FreePorts  int    `json:"free_ports"`
	TotalPorts int    `json:"total_ports"`
}
