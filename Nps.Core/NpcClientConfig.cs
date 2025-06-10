using System.Collections.Generic;
using System.Text.Json.Serialization;

using Nps.Core.Serialization;

namespace Nps.Core.Config
{
    public class NpcClientConfig
    {
        public CommonConfig? Common { get; set; }

        [JsonConverter(typeof(TunnelListJsonConverter))]
        public List<TunnelConfig> Tunnels { get; set; } = new List<TunnelConfig>();
        public List<HealthCheckConfig> HealthChecks { get; set; } = new List<HealthCheckConfig>();
    }

    public class CommonConfig
    {
        public string? ServerAddr { get; set; }
        public string? ConnType { get; set; }
        public string? Vkey { get; set; }
        public bool AutoReconnection { get; set; }
        public string? LogLevel { get; set; }
        public string? RateLimit { get; set; } // e.g., "10M"
        public string? FlowStoreDriver { get; set; }
        public string? MaxConn { get; set; }
        public string? MaxTunnelNum { get; set; }
        public string? ProxyUrl { get; set; }
        public bool TlsEnable { get; set; }
        public string? TlsVerify { get; set; } // true/false or path to CA
        public string? TlsCertFile { get; set; }
        public string? TlsKeyFile { get; set; }
        public string? TlsServerName { get; set; }
        public bool MultiAccount { get; set; }
        public string? ConfigFile { get; set; }
        public string? HeartbeatInterval { get; set; }
        public string? HeartbeatTimeout { get; set; }
        public string? ClientSecret { get; set; }
        public string? ClientId { get; set; }
        public string? NpHttpEnable { get; set; } // true/false
        public string? NpHttpServer { get; set; }
        public string? NpHttpUsername { get; set; }
        public string? NpHttpPassword { get; set; }
    }

    [JsonDerivedType(typeof(TcpTunnelConfig), typeDiscriminator: "tcp")]
    [JsonDerivedType(typeof(UdpTunnelConfig), typeDiscriminator: "udp")]
    [JsonDerivedType(typeof(Socks5TunnelConfig), typeDiscriminator: "socks5")]
    [JsonDerivedType(typeof(HttpProxyTunnelConfig), typeDiscriminator: "httpProxy")]
    [JsonDerivedType(typeof(SecretTunnelConfig), typeDiscriminator: "secret")]
    [JsonDerivedType(typeof(P2pTunnelConfig), typeDiscriminator: "p2p")]
    [JsonDerivedType(typeof(FileTunnelConfig), typeDiscriminator: "file")]
    public abstract class TunnelConfig
    {
        public string? Mode { get; set; }
        public string? TargetAddr { get; set; } // Target address and port (e.g., "127.0.0.1:80")
        public int ServerPort { get; set; } // Port on the NPS server to expose
        public string? Remark { get; set; }
        public string? ClientId { get; set; } // For multi-account, associate with specific client_id
        public string? FlowLimit { get; set; } // e.g., "1G" or "10M"
        public string? MaxConn { get; set; }
        public string? Password { get; set; } // Used by multiple tunnel types
        public string? LocalPath { get; set; } // Used by file tunnel
        public string? StripPre { get; set; } // Used by file tunnel
        public string? TlsEnable { get; set; } // true/false
        public string? ProxyUrl { get; set; } // Used by socks5, httpProxy
        public string? HealthCheckType { get; set; } // tcp, http
        public string? HealthCheckTarget { get; set; } // ip:port
        public string? HealthCheckTimeout { get; set; } // seconds
        public string? HealthCheckInterval { get; set; } // seconds
        public string? HealthCheckFailMax { get; set; } // count
        public string? HealthCheckHttpUrl { get; set; } // for http health check
        public string? HealthCheckHttpHost { get; set; } // for http health check
        public string? Username { get; set; } // for httpProxy, socks5
        public string? AllowIps { get; set; } // comma-separated list of allowed IPs
    }

    public class TcpTunnelConfig : TunnelConfig
    {
        // TCP specific properties if any in future
    }

    public class UdpTunnelConfig : TunnelConfig
    {
        // UDP specific properties
    }

    public class Socks5TunnelConfig : TunnelConfig
    {
        // Socks5 specific properties
    }

    public class HttpProxyTunnelConfig : TunnelConfig
    {
        // HTTP Proxy specific properties
    }

    public class SecretTunnelConfig : TunnelConfig
    {
        // Secret tunnel specific properties
    }

    public class P2pTunnelConfig : TunnelConfig
    {
        public string? PeerUsername { get; set; }
        public string? PeerPassword { get; set; }
    }

    public class FileTunnelConfig : TunnelConfig
    {
        // File tunnel specific properties (LocalPath, StripPre already in base)
    }


    public class HealthCheckConfig
    {
        public string? Type { get; set; } // e.g., "tcp", "http"
        public string? Target { get; set; } // Address and port to check (e.g., "127.0.0.1:8080")
        public int Timeout { get; set; } // Seconds
        public int Interval { get; set; } // Seconds
        public int MaxFailures { get; set; } // Number of failures before considering down
        public string? HttpUrl { get; set; } // For HTTP checks
        public string? HttpHost { get; set; } // For HTTP checks
        public string? Task { get; set; } // Associated task/tunnel remark or ID
    }
}
