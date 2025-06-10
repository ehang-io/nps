using System;
using System.Net.Sockets;
using System.Threading;

namespace Nps.Core.Sessions
{
    public class ClientSession : IDisposable
    {
        public string ClientId { get; }
        public string? Version { get; set; } // Full version from client
        public string? CoreVersion { get; set; } // Core version from client

        public Stream? SignalStream { get; set; }
        public Stream? MuxStream { get; set; } // Placeholder, will be replaced by MUX handler later

        public DateTime LastHeartbeat { get; set; }
        public CancellationTokenSource Cts { get; }

        private bool _disposed = false;

        public ClientSession(string clientId)
        {
            ClientId = clientId ?? throw new ArgumentNullException(nameof(clientId));
            Cts = new CancellationTokenSource();
            LastHeartbeat = DateTime.UtcNow; // Initialize to now
        }

        public void UpdateHeartbeat()
        {
            LastHeartbeat = DateTime.UtcNow;
        }

        public void Close()
        {
            if (!_disposed)
            {
                if (!Cts.IsCancellationRequested)
                {
                    Cts.Cancel();
                }

                // Close streams carefully, they might be null or already closed
                // The owner of the session should log any errors during stream closure if necessary.
                try { SignalStream?.Close(); SignalStream?.Dispose(); } catch { /* Ignore */ }
                try { MuxStream?.Close(); MuxStream?.Dispose(); } catch { /* Ignore */ }

                Cts.Dispose();
                _disposed = true;
            }
        }

        public void Dispose()
        {
            Close();
            GC.SuppressFinalize(this);
        }
    }
}
