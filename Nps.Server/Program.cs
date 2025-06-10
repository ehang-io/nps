using System;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Nps.Core.Config;
using Nps.Server.Security;

namespace Nps.Server
{
    class Program
    {
        static async Task Main(string[] args)
        {
            var loggerForPreHost = CreatePreHostLogger();

            if (args.Length > 0 && HandleServiceCommands(args, loggerForPreHost))
            {
                return; // Command was handled, exit
            }

            var hostBuilder = Host.CreateDefaultBuilder(args)
                .ConfigureAppConfiguration((hostingContext, config) =>
                {
                    config.SetBasePath(Directory.GetCurrentDirectory())
                          .AddJsonFile("appsettings.nps.json", optional: false, reloadOnChange: true)
                          .AddEnvironmentVariables()
                          .AddCommandLine(args);
                })
                .ConfigureLogging((hostingContext, logging) =>
                {
                    logging.AddConfiguration(hostingContext.Configuration.GetSection("Logging"));
                    logging.AddConsole();
                    // Add other providers like Debug, EventSource, EventLog as needed
                })
                .ConfigureServices((hostContext, services) =>
                {
                    // Bind NpsServerConfig from configuration and register it
                    var serverConfig = new NpsServerConfig();
                    hostContext.Configuration.Bind(serverConfig);
                    services.AddSingleton(serverConfig);
                    services.AddSingleton<IVKeyAuthenticator, InMemoryVKeyAuthenticator>();
                    services.AddHostedService<ServerService>();
                });

            if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
            {
                hostBuilder.UseWindowsService(options =>
                {
                    options.ServiceName = "NPS .NET Server"; // Set your desired service name
                });
            }
            else if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                hostBuilder.UseSystemd();
            }

            var host = hostBuilder.Build();
            var mainLogger = host.Services.GetRequiredService<ILogger<Program>>(); // Logger for post-host build operations

            LogEffectiveConfig(mainLogger, host.Services.GetRequiredService<NpsServerConfig>());

            mainLogger.LogInformation("Attempting to run as a service/daemon if not manually started or service command was issued.");
            await host.RunAsync();
        }

        private static bool HandleServiceCommands(string[] args, ILogger<Program> logger)
        {
            // Basic command handling for service operations
            // In a real scenario, this would interact with the service control manager (Windows)
            // or systemd (Linux), possibly using a library to simplify service installation.
            if (args.Contains("install", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service install command received. (Note: Actual installation not yet implemented. Run as a service/daemon directly or use OS tools like sc.exe or systemctl).");
                // Placeholder for actual install logic
                return true;
            }
            if (args.Contains("uninstall", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service uninstall command received. (Note: Actual uninstallation not yet implemented.)");
                // Placeholder for actual uninstall logic
                return true;
            }
            if (args.Contains("start", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service start command received. (Note: This application will run as a service if started by the OS service manager.)");
                // This command is usually handled by the SCM
                return false; // Allow host to run
            }
            if (args.Contains("stop", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service stop command received. (Note: Services are stopped via the OS service manager.)");
                // This command is usually handled by the SCM
                return true;
            }
            if (args.Contains("status", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service status command received. (Note: Service status is checked via the OS service manager.)");
                // Placeholder for status logic
                return true;
            }
            return false; // No specific service command handled
        }

        private static ILogger<Program> CreatePreHostLogger()
        {
            // Create a logger for use before the host is built (e.g., for command-line parsing)
            // This is a simplified logger setup.
            return LoggerFactory.Create(builder =>
            {
                builder.AddConsole().SetMinimumLevel(LogLevel.Information);
            }).CreateLogger<Program>();
        }

        private static void LogEffectiveConfig(ILogger<Program> logger, NpsServerConfig serverConfig)
        {
            logger.LogInformation("NPS Server starting with effective configuration...");
            logger.LogInformation("RunMode: {RunMode}", serverConfig.RunMode);
            logger.LogInformation("BridgeType: {BridgeType}", serverConfig.BridgeType);
            logger.LogInformation("BridgePort: {BridgePort}", serverConfig.BridgePort); // This will reflect command-line override if provided

            if (serverConfig.Web != null)
            {
                logger.LogInformation("Web Host: {WebHost}", serverConfig.Web.Host);
                logger.LogInformation("Web Port: {WebPort}", serverConfig.Web.Port);
            }
            else
            {
                logger.LogWarning("Web configuration (Web.*) is not fully specified in appsettings.");
            }

            if (!string.IsNullOrEmpty(serverConfig.BridgeTlsCertPath))
            {
                logger.LogInformation("Bridge TLS Certificate Path: {BridgeTlsCertPath}", serverConfig.BridgeTlsCertPath);
            }
            else
            {
                logger.LogInformation("Bridge TLS Certificate Path: Not configured (TLS on bridge disabled).");
            }
        }
    }
}
