using System;
using System.Collections.Concurrent;
using System.IO;
using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography.X509Certificates;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Nps.Core; // For VersionInfo
using Nps.Core.Config;
using Nps.Core.Protocol; // For WorkTypes, ProtocolConstants
using Nps.Core.Protocol.Utils; // For NetworkHelper
using Nps.Core.Security; // For CryptoUtils
using Nps.Core.Sessions;
using Nps.Server.Security; // For IVKeyAuthenticator

namespace Nps.Server
{
    public class ServerService : IHostedService
    {
        private readonly NpsServerConfig _config;
        private readonly ILogger<ServerService> _logger;
        private readonly IVKeyAuthenticator _vKeyAuthenticator;
        private TcpListener? _tcpListener;
        private CancellationTokenSource? _cancellationTokenSource;
        private X509Certificate2? _serverCertificate;
        private readonly ConcurrentDictionary<string, ClientSession> _clientSessions = new();

        public ServerService(NpsServerConfig config, ILogger<ServerService> logger, IVKeyAuthenticator vKeyAuthenticator)
        {
            _config = config ?? throw new ArgumentNullException(nameof(config));
            _logger = logger ?? throw new ArgumentNullException(nameof(logger));
            _vKeyAuthenticator = vKeyAuthenticator ?? throw new ArgumentNullException(nameof(vKeyAuthenticator));
        }

        public async Task StartAsync(CancellationToken cancellationToken) // IHostedService provides the token
        {
            if (_config.BridgePort <= 0)
            {
                _logger.LogError("BridgePort is not configured correctly. Server cannot start.");
                return;
            }

            IPAddress ipAddress = string.IsNullOrEmpty(_config.Bridge?.Ip) || _config.Bridge.Ip == "0.0.0.0"
                ? IPAddress.Any
                : IPAddress.Parse(_config.Bridge.Ip);

            _tcpListener = new TcpListener(ipAddress, _config.BridgePort);
            // Use the cancellationToken from IHostedService.StartAsync for the CancellationTokenSource
            _cancellationTokenSource = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);


            try
            {
                _tcpListener.Start();
                _logger.LogInformation("NPS Server bridge started on {IPAddress}:{Port}", ipAddress, _config.BridgePort);
                _logger.LogInformation("Effective BridgePort used: {BridgePort}", _config.BridgePort);

                LoadServerCertificate();

                while (!_cancellationTokenSource.Token.IsCancellationRequested)
                {
                    try
                    {
                        _logger.LogDebug("Waiting for a client connection...");
                        TcpClient tcpClient = await _tcpListener.AcceptTcpClientAsync(_cancellationTokenSource.Token);
                        _logger.LogInformation("Client attempting to connect: {RemoteEndPoint}", tcpClient.Client.RemoteEndPoint?.ToString() ?? "Unknown");

                        // Offload connection handling to not block the accept loop
                        _ = Task.Run(async () => await HandleClientConnectionAsync(tcpClient, _cancellationTokenSource.Token), _cancellationTokenSource.Token)
                            .ContinueWith(t =>
                            {
                                if(t.IsFaulted && t.Exception != null)
                                {
                                   _logger.LogError(t.Exception, "Unhandled exception in HandleClientConnectionAsync task.");
                                }
                            }, TaskContinuationOptions.OnlyOnFaulted);
                    }
                    catch (OperationCanceledException)
                    {
                        // This is expected when cancellation is requested by _cancellationTokenSource
                        _logger.LogInformation("TCP listener accept operation cancelled.");
                    }
                    catch (Exception ex)
                    {
                        if (_cancellationTokenSource.Token.IsCancellationRequested)
                        {
                             _logger.LogInformation("TCP listener shutting down due to cancellation.");
                             break;
                        }
                        _logger.LogError(ex, "Error accepting client connection");
                        // Avoid rapid looping on persistent errors if not cancelled
                        await Task.Delay(1000, _cancellationTokenSource.Token);
                    }
                }
            }
            catch (SocketException ex)
            {
                _logger.LogError(ex, "Failed to start TCP listener on {IPAddress}:{Port}. Error: {ErrorMessage}", ipAddress, _config.BridgePort, ex.Message);
            }
            catch (Exception ex)
            {
                // Catch any other exceptions during server startup (e.g. CancellationTokenSource.CreateLinkedTokenSource if cancellationToken is already cancelled)
                 if (cancellationToken.IsCancellationRequested) {
                     _logger.LogInformation("Server startup cancelled.");
                 } else {
                    _logger.LogError(ex, "An unexpected error occurred while starting or running the server.");
                 }
            }
            finally
            {
                if (_tcpListener?.Server.IsBound == true)
                {
                    _tcpListener.Stop();
                    _logger.LogInformation("NPS Server bridge stopped.");
                }
            }
        }

        public Task StopAsync(CancellationToken cancellationToken)
        {
            _logger.LogInformation("NPS Server service stopping (called by host).");
            if (_cancellationTokenSource != null && !_cancellationTokenSource.IsCancellationRequested)
            {
                _cancellationTokenSource.Cancel();
            }
            return Task.CompletedTask;
        }

        private void LoadServerCertificate()
        {
            if (string.IsNullOrEmpty(_config.BridgeTlsCertPath))
            {
                _logger.LogInformation("BridgeTlsCertPath is not configured. Server will run without TLS on the bridge.");
                return;
            }

            string certPath = Path.GetFullPath(_config.BridgeTlsCertPath);
            _logger.LogDebug("Attempting to load server certificate from: {Path}", certPath);

            if (!File.Exists(certPath))
            {
                _logger.LogWarning("Server certificate file not found at {Path}. Server will run without TLS on the bridge.", certPath);
                return;
            }

            try
            {
                _serverCertificate = new X509Certificate2(certPath, _config.BridgeTlsCertPassword);
                _logger.LogInformation("Server certificate loaded successfully from {Path}. Thumbprint: {Thumbprint}", certPath, _serverCertificate.Thumbprint);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to load server certificate from {Path}. Server will run without TLS on the bridge.", certPath);
                _serverCertificate = null;
            }
        }

        private async Task HandleClientConnectionAsync(TcpClient tcpClient, CancellationToken cancellationToken)
        {
            string remoteEndPointStr = tcpClient.Client.RemoteEndPoint?.ToString() ?? "Unknown";
            _logger.LogDebug("Handling connection from {RemoteEndPoint}", remoteEndPointStr);

            NetworkStream baseStream = tcpClient.GetStream();
            Stream streamToUse = baseStream;

            try
            {
                if (_serverCertificate != null)
                {
                    _logger.LogDebug("Attempting TLS handshake with client {RemoteEndPoint}", remoteEndPointStr);
                    SslStream sslStream = new SslStream(baseStream, leaveInnerStreamOpen: false);
                    // Use a separate CancellationToken for the handshake with a timeout, if desired
                    // For example: CancellationTokenSource handshakeCts = new CancellationTokenSource(TimeSpan.FromSeconds(10));
                    // CancellationToken linkedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, handshakeCts.Token).Token;
                    await sslStream.AuthenticateAsServerAsync(
                        new SslServerAuthenticationOptions
                        {
                            ServerCertificate = _serverCertificate,
                            ClientCertificateRequired = false,
                            EnabledSslProtocols = SslProtocols.Tls12 | SslProtocols.Tls13,
                            CertificateRevocationCheckMode = X509RevocationMode.NoCheck // Or Online/Offline depending on needs
                        }, cancellationToken); // Use the overall connection token for now

                    _logger.LogInformation("TLS handshake successful with {RemoteEndPoint}. Protocol: {SslProtocol}, Cipher: {CipherAlgorithm} {CipherStrength}",
                        remoteEndPointStr, sslStream.SslProtocol, sslStream.CipherAlgorithm, sslStream.CipherStrength);
                    streamToUse = sslStream;
                }
                else
                {
                    _logger.LogInformation("Proceeding with unencrypted connection for {RemoteEndPoint} (TLS certificate not loaded/configured).", remoteEndPointStr);
                }

                // The NetworkHelper methods expect NetworkStream. SslStream does not inherit from NetworkStream.
                // This requires a decision:
                // 1. Modify NetworkHelper to accept Stream and perform checks/casts. (More robust)
                // 2. Assume SslStream behaves "close enough" for these specific byte operations if passed as Stream. (Risky)
                // 3. Create SslStream specific helpers or use SslStream methods directly.
                // For now, let's try to make NetworkHelper more generic by accepting Stream where possible,
                // or acknowledge that direct NetworkStream operations might fail on SslStream.
                // The cast `streamToUse as NetworkStream ?? baseStream` is problematic because SslStream isn't a NetworkStream.
                // The correct underlying stream for SslStream is its InnerStream, but that's already wrapped.
                // For simplicity in this step, we'll assume that the SslStream will correctly pass through
                // the ReadAsync/WriteAsync calls to the underlying NetworkStream for byte-level operations.
                // This is generally true for SslStream.

                if (!await NetworkHelper.VerifyConnTestAsync(streamToUse, cancellationToken))
                {
                    _logger.LogWarning("CONN_TEST verification failed for {RemoteEndPoint}.", remoteEndPointStr);
                    tcpClient.Close(); return;
                }
                _logger.LogDebug("CONN_TEST verified for {RemoteEndPoint}.", remoteEndPointStr);

                string? clientCoreVersion = await NetworkHelper.ReadStringWithLengthPrefixAsync(streamToUse, cancellationToken);
                string? clientFullVersion = await NetworkHelper.ReadStringWithLengthPrefixAsync(streamToUse, cancellationToken);
                _logger.LogInformation("Client {RemoteEndPoint} versions: Core='{ClientCoreVersion}', Full='{ClientFullVersion}'", remoteEndPointStr, clientCoreVersion, clientFullVersion);

                byte[] serverCoreVersionMd5 = CryptoUtils.ToMd5HashBytes(VersionInfo.CoreVersion);
                await NetworkHelper.WriteBytesWithLengthPrefixAsync(streamToUse, serverCoreVersionMd5, cancellationToken);

                byte[]? clientVKeyMd5Bytes = await NetworkHelper.ReadFixedLengthBytesAsync(streamToUse, 16, cancellationToken);
                if (clientVKeyMd5Bytes == null) { _logger.LogWarning("Failed to read VKey MD5 from {RemoteEndPoint}.", remoteEndPointStr); tcpClient.Close(); return; }
                string clientVKeyMd5Hash = BitConverter.ToString(clientVKeyMd5Bytes).Replace("-", "").ToLowerInvariant();
                _logger.LogDebug("Received VKey MD5 Hash {ClientVKeyMd5Hash} from {RemoteEndPoint}", clientVKeyMd5Hash, remoteEndPointStr);

                string? remoteIp = tcpClient.Client.RemoteEndPoint is IPEndPoint ipEp ? ipEp.Address.ToString() : null;
                string? clientId = await _vKeyAuthenticator.AuthenticateAsync(clientVKeyMd5Hash, remoteIp);

                if (clientId == null)
                {
                    _logger.LogWarning("VKey authentication failed for {RemoteEndPoint} with VKey hash {ClientVKeyMd5Hash}.", remoteEndPointStr, clientVKeyMd5Hash);
                    await NetworkHelper.WriteFlagAsync(streamToUse, WorkTypes.VerifyErr, cancellationToken);
                    tcpClient.Close(); return;
                }
                await NetworkHelper.WriteFlagAsync(streamToUse, WorkTypes.VerifySuccess, cancellationToken);
                _logger.LogInformation("Client {RemoteEndPoint} authenticated successfully. ClientId: {ClientId}", remoteEndPointStr, clientId);

                ClientSession session = _clientSessions.GetOrAdd(clientId, id => new ClientSession(id));
                session.CoreVersion = clientCoreVersion;
                session.Version = clientFullVersion;
                session.UpdateHeartbeat();

                string? workType = await NetworkHelper.ReadFlagAsync(streamToUse, cancellationToken);
                _logger.LogDebug("Received work type '{WorkType}' from client {ClientId} ({RemoteEndPoint})", workType, clientId, remoteEndPointStr);

                // Mark the stream as "taken" so the finally block doesn't close it.
                bool streamTaken = false;
                switch (workType)
                {
                    case WorkTypes.WorkMain:
                        if (session.SignalStream != null)
                        {
                            _logger.LogWarning("Client {ClientId} attempted to establish a duplicate WorkMain signal channel. Closing new connection.", clientId);
                            tcpClient.Close(); // Close the new connection, keep the old one.
                            return;
                        }
                        session.SignalStream = streamToUse;
                        streamTaken = true;
                        _logger.LogInformation("Signal channel (WorkMain) established for client {ClientId} ({RemoteEndPoint}).", clientId, remoteEndPointStr);
                        await HandleSignalChannelAsync(session);
                        break;

                    case WorkTypes.WorkChan:
                        if (session.MuxStream != null)
                        {
                            _logger.LogWarning("Client {ClientId} attempted to establish a duplicate WorkChan MUX channel. Closing new connection.", clientId);
                            tcpClient.Close(); // Close the new connection, keep the old one.
                            return;
                        }
                        session.MuxStream = streamToUse;
                        streamTaken = true;
                        _logger.LogInformation("MUX channel (WorkChan) established for client {ClientId} ({RemoteEndPoint}).", clientId, remoteEndPointStr);
                        await HandleMuxChannelAsync(session);
                        break;

                    default:
                        _logger.LogWarning("Unknown work type '{WorkType}' received from client {ClientId} ({RemoteEndPoint}). Closing connection.", workType, clientId, remoteEndPointStr);
                        break;
                }
                // If stream was taken, ensure this method doesn't close tcpClient by returning.
                if(streamTaken) return;
            }
            catch (OperationCanceledException) { _logger.LogInformation("HandleClientConnectionAsync cancelled for {RemoteEndPoint}.", remoteEndPointStr); }
            catch (IOException ex) { _logger.LogError(ex, "IOException in HandleClientConnectionAsync for {RemoteEndPoint}: {ErrorMessage}", remoteEndPointStr, ex.Message); }
            catch (Exception ex) { _logger.LogError(ex, "Unexpected error in HandleClientConnectionAsync for {RemoteEndPoint}", remoteEndPointStr); }
            finally
            {
                // Close only if the stream wasn't passed off to another handler
                // This logic needs to be more robust; streamTaken helps but is error-prone with multiple exit points.
                // A "using" pattern or explicit ownership transfer is better.
                // For now, if an exception occurred before streamTaken=true, or if worktype was unknown:
                if (tcpClient.Connected) // Check if still connected before trying to close
                {
                    _logger.LogDebug("Closing client connection in HandleClientConnectionAsync finally block for {RemoteEndPoint}", remoteEndPointStr);
                    tcpClient.Close();
                }
            }
        }

        private async Task HandleSignalChannelAsync(ClientSession session)
        {
            if (session.SignalStream == null)
            {
                _logger.LogError("SignalStream is null for client {ClientId} at the start of HandleSignalChannelAsync.", session.ClientId);
                CheckAndCleanupSession(session.ClientId);
                return;
            }
            _logger.LogInformation("Signal channel handler started for client {ClientId}.", session.ClientId);
            try
            {
                while (!session.Cts.Token.IsCancellationRequested && session.SignalStream.CanRead)
                {
                    await Task.Delay(TimeSpan.FromSeconds(15), session.Cts.Token);
                    session.UpdateHeartbeat();
                    _logger.LogDebug("Signal channel for {ClientId} still active. Last heartbeat: {LastHeartbeat}", session.ClientId, session.LastHeartbeat);
                }
            }
            catch (OperationCanceledException) { _logger.LogInformation("Signal channel for client {ClientId} cancelled.", session.ClientId); }
            catch (IOException ex) { _logger.LogError(ex, "IOException in signal channel for client {ClientId}.", session.ClientId); }
            catch (Exception ex) { _logger.LogError(ex, "Error in signal channel for client {ClientId}.", session.ClientId); }
            finally
            {
                _logger.LogInformation("Signal channel for client {ClientId} closing.", session.ClientId);
                session.SignalStream?.Close(); // It's a NetworkStream, Close also disposes.
                session.SignalStream = null;
                CheckAndCleanupSession(session.ClientId);
            }
        }

        private async Task HandleMuxChannelAsync(ClientSession session)
        {
             if (session.MuxStream == null)
            {
                _logger.LogError("MuxStream is null for client {ClientId} at the start of HandleMuxChannelAsync.", session.ClientId);
                CheckAndCleanupSession(session.ClientId);
                return;
            }
            _logger.LogInformation("MUX channel handler started for client {ClientId}.", session.ClientId);
            try
            {
                while (!session.Cts.Token.IsCancellationRequested && session.MuxStream.CanRead)
                {
                    await Task.Delay(TimeSpan.FromSeconds(30), session.Cts.Token);
                    _logger.LogDebug("MUX channel for {ClientId} still active.", session.ClientId);
                }
            }
            catch (OperationCanceledException) { _logger.LogInformation("MUX channel for client {ClientId} cancelled.", session.ClientId); }
            catch (IOException ex) { _logger.LogError(ex, "IOException in MUX channel for client {ClientId}.", session.ClientId); }
            catch (Exception ex) { _logger.LogError(ex, "Error in MUX channel for client {ClientId}.", session.ClientId); }
            finally
            {
                _logger.LogInformation("MUX channel for client {ClientId} closing.", session.ClientId);
                session.MuxStream?.Close();
                session.MuxStream = null;
                CheckAndCleanupSession(session.ClientId);
            }
        }

        private void CheckAndCleanupSession(string clientId)
        {
            if (_clientSessions.TryGetValue(clientId, out var session))
            {
                if (session.SignalStream == null && session.MuxStream == null)
                {
                    _logger.LogInformation("Both channels for client {ClientId} are closed. Removing session.", clientId);
                    session.Close();
                    _clientSessions.TryRemove(clientId, out _);
                }
            }
        }
    }
}
