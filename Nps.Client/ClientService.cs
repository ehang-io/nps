using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Nps.Core.Config;

namespace Nps.Client
{
    public class ClientService : IHostedService
    {
        private readonly NpcClientConfig _config;
        private readonly ILogger<ClientService> _logger;
        private CancellationTokenSource? _internalCts;

        public ClientService(NpcClientConfig config, ILogger<ClientService> logger)
        {
            _config = config;
            _logger = logger;
        }

        public Task StartAsync(CancellationToken cancellationToken)
        {
            _logger.LogInformation("NPC Client service starting...");
            _logger.LogInformation("Server Address: {ServerAddr}, VKey: {Vkey}", _config.Common?.ServerAddr, _config.Common?.Vkey);

            if (_config.Tunnels != null)
            {
                _logger.LogInformation("Configured tunnels: {TunnelCount}", _config.Tunnels.Count);
                foreach(var tunnel in _config.Tunnels)
                {
                    _logger.LogDebug("  Tunnel: Mode={Mode}, ServerPort={ServerPort}, Target={TargetAddr}", tunnel.Mode, tunnel.ServerPort, tunnel.TargetAddr);
                }
            }

            _internalCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);

            // Placeholder for actual client connection logic loop
            // For example, you might start a manager here that handles multiple tunnel connections
            // Task.Run(async () => await ConnectAndManageTunnelsAsync(_internalCts.Token), _internalCts.Token);
            _logger.LogInformation("Client service main logic placeholder started. (Actual tunnel connections not yet implemented).");


            if (_config.Common?.AutoReconnection == true)
            {
                _logger.LogInformation("Auto-reconnection is enabled.");
            }

            return Task.CompletedTask;
        }

        public Task StopAsync(CancellationToken cancellationToken)
        {
            _logger.LogInformation("NPC Client service stopping...");
            if(_internalCts != null && !_internalCts.IsCancellationRequested)
            {
                _internalCts.Cancel();
            }
            // Placeholder for actual disconnection logic
            _logger.LogInformation("Client service cleanup placeholder. (Actual tunnel disconnections not yet implemented).");
            return Task.CompletedTask;
        }

        // Example of a method that would contain the core client logic
        // private async Task ConnectAndManageTunnelsAsync(CancellationToken cancellationToken)
        // {
        //     while(!cancellationToken.IsCancellationRequested)
        //     {
        //          _logger.LogDebug("Client management loop running...");
        //          // Connect to server
        //          // For each tunnel in config, establish connection
        //          // Handle reconnections if AutoReconnection is true
        //          await Task.Delay(5000, cancellationToken); // Example delay
        //     }
        //     _logger.LogInformation("Client management loop stopped.");
        // }
    }
}
