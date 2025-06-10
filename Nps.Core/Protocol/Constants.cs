namespace Nps.Core.Protocol
{
    public static class ProtocolConstants
    {
        // Represents the Go constant common.CONN_TEST = "4V"
        // Decimal values: 18, 52, 86
        // Hex values: 0x12, 0x34, 0x56
        public static readonly byte[] ConnTestBytes = new byte[] { 0x12, 0x34, 0x56 };

        // It might be useful to have the string representation as well,
        // but care must be taken with encoding if it's used directly.
        // For matching network bytes, ConnTestBytes is primary.
        public const string ConnTestString = "\u00124V";

        // Other protocol-wide constants can be added here.
        // For example, default buffer sizes, timeout values not meant for configuration, etc.
        public const int DefaultBufferSize = 4096; // Example
    }
}
