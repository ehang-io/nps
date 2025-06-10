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

namespace Nps.Client
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
                          .AddJsonFile("appsettings.npc.json", optional: false, reloadOnChange: true)
                          .AddEnvironmentVariables()
                          .AddCommandLine(args);
                })
                .ConfigureLogging((hostingContext, logging) =>
                {
                    // Configure logging from appsettings.json
                    // For client, might want to add a default "Logging" section to appsettings.npc.json
                    // similar to Nps.Server if not already present, or set defaults here.
                    var loggingConfiguration = hostingContext.Configuration.GetSection("Logging");
                    if (!loggingConfiguration.Exists()) // Check if Logging section exists
                    {
                        // Apply some defaults if no configuration is found
                        logging.AddSimpleConsole(options =>
                        {
                            options.TimestampFormat = "HH:mm:ss ";
                            options.SingleLine = true;
                        });
                        logging.SetMinimumLevel(LogLevel.Information); // Default to Information if not set
                        loggerForPreHost.LogWarning("No 'Logging' section found in appsettings.npc.json. Applying default simple console logger with Information level.");
                    }
                    else
                    {
                         logging.AddConfiguration(hostingContext.Configuration.GetSection("Logging"));
                         logging.AddConsole();
                    }
                })
                .ConfigureServices((hostContext, services) =>
                {
                    var clientConfig = new NpcClientConfig();
                    hostContext.Configuration.Bind(clientConfig);
                    services.AddSingleton(clientConfig);

                    services.AddHostedService<ClientService>();
                });

            if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
            {
                hostBuilder.UseWindowsService(options =>
                {
                    options.ServiceName = "NPC .NET Client";
                });
            }
            else if (RuntimeInformation.IsOSPlatform(OSPlatform.Linux))
            {
                hostBuilder.UseSystemd();
            }

            var host = hostBuilder.Build();
            var mainLogger = host.Services.GetRequiredService<ILogger<Program>>();

            LogEffectiveConfig(mainLogger, host.Services.GetRequiredService<NpcClientConfig>());
            mainLogger.LogInformation("Attempting to run as a service/daemon if not manually started or service command was issued.");

            await host.RunAsync();
        }

        private static bool HandleServiceCommands(string[] args, ILogger<Program> logger)
        {
            if (args.Contains("install", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service install command received. (Note: Actual installation not yet implemented.)");
                return true;
            }
            if (args.Contains("uninstall", StringComparer.OrdinalIgnoreCase))
            {
                logger.LogInformation("Service uninstall command received. (Note: Actual uninstallation not yet implemented.)");
                return true;
            }
            return false;
        }

        private static ILogger<Program> CreatePreHostLogger()
        {
            return LoggerFactory.Create(builder =>
            {
                builder.AddConsole().SetMinimumLevel(LogLevel.Information);
            }).CreateLogger<Program>();
        }

        private static void LogEffectiveConfig(ILogger<Program> logger, NpcClientConfig clientConfig)
        {
            logger.LogInformation("NPC Client starting with effective configuration...");
            if (clientConfig.Common != null)
            {
                logger.LogInformation("Common - ServerAddr: {ServerAddr}", clientConfig.Common.ServerAddr);
                logger.LogInformation("Common - VKey: {Vkey}", clientConfig.Common.Vkey);
                logger.LogInformation("Common - ConnType: {ConnType}", clientConfig.Common.ConnType);
                logger.LogInformation("Common - AutoReconnection: {AutoReconnection}", clientConfig.Common.AutoReconnection);
            }
            else
            {
                logger.LogWarning("Common configuration section is missing!");
            }

            if (clientConfig.Tunnels != null && clientConfig.Tunnels.Any())
            {
                logger.LogInformation("Tunnels configured: {TunnelCount}", clientConfig.Tunnels.Count);
                // Optionally log details for each tunnel if desired, respecting log levels
            }
            else
            {
                logger.LogInformation("No tunnels configured.");
            }
        }
    }
}
