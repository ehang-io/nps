using System.Threading.Tasks;

namespace Nps.Server.Security
{
    public interface IVKeyAuthenticator
    {
        /// <summary>
        /// Authenticates a VKey hash.
        /// </summary>
        /// <param name="vkeyMd5Hash">The MD5 hash of the VKey provided by the client.</param>
        /// <param name="remoteIp">The remote IP address of the client.</param>
        /// <returns>A client ID string if authentication is successful; otherwise, null.</returns>
        Task<string?> AuthenticateAsync(string vkeyMd5Hash, string? remoteIp);
    }
}
