namespace Nps.Core.Protocol
{
    public static class WorkTypes
    {
        // Connection establishment and verification
        public const string ConnTest = "t";       // Test connection viability (may not be a work type but an initial handshake byte)
        public const string VerifyReq = "v";      // Request from client to verify VKey / credentials
        public const string VerifyErr = "e";      // Verification error from server
        public const string VerifySuccess = "s";  // Verification success from server

        // Core work types for data tunneling / proxying
        public const string WorkMain = "m";       // Main work connection (e.g., for TCP, UDP proxy)
        public const string WorkChan = "c";       // Channel for data transfer within a main work connection (e.g., new TCP connection for a tunnel)

        // Configuration and control messages
        public const string WorkConfig = "g";     // Client requests configuration from server
        public const string NewConf = "o";        // Client sends new configuration to server (e.g. from npc.conf)
        public const string NewHost = "h";        // Client sends new host information to server (part of config sync)
        public const string NewTask = "a";        // Client sends new task (tunnel) information (part of config sync)
                                                  // Server might also send this to client if server-side changes dictate a new task for client? (Needs clarification)

        // Specific tunnel types / features
        public const string WorkRegister = "r";   // Client registers itself (e.g. for P2P or specific auth)
        public const string WorkSecret = "x";     // Secret tunnel related work
        public const string WorkFile = "f";       // File tunnel related work
        public const string WorkP2p = "p";        // P2P tunnel related work

        // UDP specific
        public const string NewUdpConn = "u";     // Server signals client to establish a new UDP connection for a tunnel

        // Status and Keep-alive
        public const string WorkStatus = "w";     // Client requests status from server / sends its status
        public const string KeepAlive = "k";      // Keep-alive packet (not in original list, but common)

        // Placeholder for other types if discovered
        // Example: Client disconnect signal, error messages etc.
        // public const string Disconnect = "d";
    }
}
