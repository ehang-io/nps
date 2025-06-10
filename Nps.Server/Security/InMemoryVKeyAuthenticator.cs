using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Nps.Core.Config;
using Nps.Core.Security; // For CryptoUtils

namespace Nps.Server.Security
{
    public class InMemoryVKeyAuthenticator : IVKeyAuthenticator
    {
        private readonly NpsServerConfig _config;
        private readonly ILogger<InMemoryVKeyAuthenticator> _logger;
        private readonly string? _serverPublicVKeyMd5;

        public InMemoryVKeyAuthenticator(NpsServerConfig config, ILogger<InMemoryVKeyAuthenticator> logger)
        {
            _config = config;
            _logger = logger;

            if (!string.IsNullOrEmpty(_config.PublicVkey))
            {
                _serverPublicVKeyMd5 = CryptoUtils.ToMd5Hash(_config.PublicVkey);
                _logger.LogDebug("Server PublicVkey configured. MD5 Hash: {ServerPublicVkeyMd5}", _serverPublicVKeyMd5);
            }
            else
            {
                _logger.LogWarning("PublicVkey is not configured in NpsServerConfig. VKey authentication will fail unless client sends an empty VKey (if allowed).");
            }
        }

        public Task<string?> AuthenticateAsync(string clientVKeyMd5Hash, string? remoteIp)
        {
            // For now, we only support a single public VKey.
            // Later, this can be expanded to check against a list of configured clients.
            if (string.IsNullOrEmpty(_serverPublicVKeyMd5))
            {
                // If server has no VKey, perhaps only clients sending no VKey (empty hash) are allowed for some modes?
                // For now, if server has no VKey, no client can authenticate.
                _logger.LogWarning("Authentication attempt from {RemoteIp} with VKey hash {ClientVKeyMd5Hash}, but server PublicVkey is not set. Authentication failed.", remoteIp, clientVKeyMd5Hash);
                return Task.FromResult<string?>(null);
            }

            if (_serverPublicVKeyMd5.Equals(clientVKeyMd5Hash, System.StringComparison.OrdinalIgnoreCase))
            {
                // For a single public VKey, we can assign a generic client ID.
                // Or, if the VKey itself should be the client ID (after verifying it's known), that's another model.
                // The original Go nps uses the VKey (from client config) as a form of client identifier for tasks.
                // Let's use a generic ID for now for the public VKey.
                string clientId = "public_client_" + clientVKeyMd5Hash.Substring(0, 8); // Create a somewhat unique ID
                _logger.LogInformation("Client {RemoteIp} authenticated successfully with VKey hash {ClientVKeyMd5Hash}. Assigned ClientId: {ClientId}", remoteIp, clientVKeyMd5Hash, clientId);
                return Task.FromResult<string?>(clientId);
            }

            _logger.LogWarning("Authentication failed for client {RemoteIp}. VKey hash mismatch. Client sent: {ClientVKeyMd5Hash}, Server expected: {ServerPublicVkeyMd5}", remoteIp, clientVKeyMd5Hash, _serverPublicVKeyMd5);
            return Task.FromResult<string?>(null);
        }
    }
}
