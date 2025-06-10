namespace Nps.Core
{
    public static class VersionInfo
    {
        // These should ideally match or be compatible with the Go version's core protocol version.
        // Example: "0.1" or similar, reflecting the core interop version.
        public const string CoreVersion = "0.26.10"; // Placeholder, align with Go's version.GetVersion() output for core features

        // This can be the specific .NET implementation's version.
        public const string FullVersion = "0.1.0-dotnet"; // Example version for this .NET port
    }
}
