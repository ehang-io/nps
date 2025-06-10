using System;

namespace Nps.Core.Protocol
{
    //[Serializable] // Only needed if using BinaryFormatter, prefer System.Text.Json
    public class LinkInformation
    {
        // Parameterless constructor for JSON deserialization
        public LinkInformation() { }

        public LinkInformation(string targetHost, string targetType, bool isEncrypted, bool isCompressed, string? clientRemoteAddr, bool isLocalProxy)
        {
            TargetHost = targetHost;
            TargetType = targetType;
            IsEncrypted = isEncrypted;
            IsCompressed = isCompressed;
            ClientRemoteAddr = clientRemoteAddr;
            IsLocalProxy = isLocalProxy;
        }

        public string? TargetHost { get; set; } // e.g., "127.0.0.1:8080"
        public string? TargetType { get; set; } // e.g., "tcp", "udp"

        // In Go code, Crypt and Compress are bools in conn.Link. Let's map them directly.
        public bool IsEncrypted { get; set; }
        public bool IsCompressed { get; set; }

        public string? ClientRemoteAddr { get; set; } // Original user's IP, if available/forwarded
        public bool IsLocalProxy { get; set; } // Indicates if the link is for a local proxy setup (e.g. socks5, http proxy on client side)

        // Consider adding other fields if they are part of the Link info exchange
        // For example, a unique ID for the link, or specific task ID it relates to.
    }
}
