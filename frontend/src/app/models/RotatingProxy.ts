export interface RotatingProxy {
  id: number;
  name: string;
  protocol: string;
  listen_protocol?: string | null;
  transport_protocol?: string | null;
  listen_transport_protocol?: string | null;
  uptime_filter_type?: string | null;
  uptime_percentage?: number | null;
  alive_proxy_count: number;
  listen_port: number;
  auth_required: boolean;
  auth_username?: string | null;
  auth_password?: string | null;
  listen_host?: string | null;
  listen_address?: string | null;
  last_rotation_at?: string | null;
  last_served_proxy?: string | null;
  reputation_labels?: string[] | null;
  created_at: string;
}

export interface CreateRotatingProxy {
  name: string;
  protocol: string;
  listen_protocol: string;
  transport_protocol: string;
  listen_transport_protocol: string;
  uptime_filter_type?: string | null;
  uptime_percentage?: number | null;
  auth_required: boolean;
  auth_username?: string | null;
  auth_password?: string | null;
  reputation_labels?: string[] | null;
}

export interface RotatingProxyNext {
  proxy_id: number;
  ip: string;
  port: number;
  username?: string | null;
  password?: string | null;
  has_auth: boolean;
  protocol: string;
}
