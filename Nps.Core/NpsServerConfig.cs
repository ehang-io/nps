using System.Collections.Generic;

namespace Nps.Core.Config
{
    public class NpsServerConfig
    {
        public string? RunMode { get; set; }
        public string? HttpProxyIp { get; set; }
        public int HttpProxyPort { get; set; }
        public string? BridgeType { get; set; }
        public int BridgePort { get; set; }
        public string? PublicVkey { get; set; }
        public string? LogLevel { get; set; }
        public WebConfig? Web { get; set; }
        public string? AuthCryptKey { get; set; }
        public int DisconnectTimeout { get; set; }
        public FlowConfig? Flow { get; set; }
        public IpLimitConfig? IpLimit { get; set; }
        public BridgeConfig? Bridge { get; set; }
        public HttpsConfig? Https { get; set; }
        public NpHttpConfig? NpHttp { get; set; }
        public string? TlsEnable { get; set; }
        public string? TlsCertFile { get; set; }
        public string? TlsKeyFile { get; set; }
        public string? Pprof { get; set; }
        public int PprofPort { get; set; }

        // TLS settings for the bridge/tunneling service
        public string? BridgeTlsCertPath { get; set; }
        public string? BridgeTlsCertPassword { get; set; }
    }

    public class WebConfig
    {
        public string? Host { get; set; }
        public string? Username { get; set; }
        public string? Password { get; set; }
        public int Port { get; set; }
        public string? Path { get; set; }
        public string? Type { get; set; }
        public bool UseCompress { get; set; }
        public bool UseCache { get; set; }
    }

    public class FlowConfig
    {
        public int StoreDriver { get; set; }
        public int ClientMaxLimit { get; set; }
        public int ServerMaxLimit { get; set; }
    }

    public class IpLimitConfig
    {
        public bool Enable { get; set; }
        public int MaxIpConnect { get; set; }
        public int MaxTunnelConnect { get; set; }
    }

    public class BridgeConfig
    {
        public string? Ip { get; set; }
        public string? Domain { get; set; }
        public string? ServerPath { get; set; }
    }

    public class HttpsConfig
    {
        public bool ProxyTlsEnable { get; set; }
        public string? ProxyTlsCertFile { get; set; }
        public string? ProxyTlsKeyFile { get; set; }
    }

    public class NpHttpConfig
    {
        public bool Enable { get; set; }
        public string? Server { get; set; }
        public string? Username { get; set; }
        public string? Password { get; set; }
        public string? ProxyUrl { get; set; }
    }
}
