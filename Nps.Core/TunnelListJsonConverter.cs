using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;
using Nps.Core.Config;

namespace Nps.Core.Serialization
{
    public class TunnelListJsonConverter : JsonConverter<List<TunnelConfig>>
    {
        public override List<TunnelConfig> Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            if (reader.TokenType != JsonTokenType.StartArray)
            {
                throw new JsonException("Expected StartArray token");
            }

            var list = new List<TunnelConfig>();
            var optionsWithoutThisConverter = new JsonSerializerOptions(options);
            optionsWithoutThisConverter.Converters.Remove(this); // Avoid re-entrancy

            while (reader.Read())
            {
                if (reader.TokenType == JsonTokenType.EndArray)
                {
                    return list;
                }

                if (reader.TokenType == JsonTokenType.StartObject)
                {
                    // Peek at the "Mode" property to determine the type
                    Utf8JsonReader readerClone = reader; // Clone to read ahead without advancing the main reader
                    string? mode = null;
                    string? currentPropertyName = null;

                    while(readerClone.Read() && readerClone.TokenType != JsonTokenType.EndObject)
                    {
                        if (readerClone.TokenType == JsonTokenType.PropertyName)
                        {
                            currentPropertyName = readerClone.GetString();
                        }
                        else if (readerClone.TokenType == JsonTokenType.String && currentPropertyName == "Mode")
                        {
                            mode = readerClone.GetString();
                            break;
                        }
                    }

                    if (string.IsNullOrEmpty(mode))
                    {
                        // This case should ideally not be hit if JSON is well-formed and Mode is present.
                        // If it can happen, one might deserialize to JsonElement first, inspect, then convert.
                        // For now, throwing an exception is clear.
                         throw new JsonException("Tunnel 'Mode' property not found, not a string, or not found early enough in the JSON object for this converter logic.");
                    }

                    TunnelConfig? tunnel = mode.ToLowerInvariant() switch
                    {
                        "tcp" => JsonSerializer.Deserialize<TcpTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "udp" => JsonSerializer.Deserialize<UdpTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "socks5" => JsonSerializer.Deserialize<Socks5TunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "httpproxy" => JsonSerializer.Deserialize<HttpProxyTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "secret" => JsonSerializer.Deserialize<SecretTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "p2p" => JsonSerializer.Deserialize<P2pTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        "file" => JsonSerializer.Deserialize<FileTunnelConfig>(ref reader, optionsWithoutThisConverter),
                        _ => throw new JsonException($"Unknown tunnel mode: {mode}")
                    };

                    if (tunnel != null)
                    {
                        list.Add(tunnel);
                    }
                    // If tunnel is null after deserialization (e.g. if the JSON object was just "null"),
                    // it's implicitly skipped, which is often desired.
                }
                else
                {
                    // Skip over other token types if necessary, or throw error
                     throw new JsonException($"Unexpected token {reader.TokenType}");
                }
            }

            throw new JsonException("Expected EndArray token");
        }

        public override void Write(Utf8JsonWriter writer, List<TunnelConfig> value, JsonSerializerOptions options)
        {
            writer.WriteStartArray();
            var optionsWithoutThisConverter = new JsonSerializerOptions(options);
            optionsWithoutThisConverter.Converters.Remove(this); // Avoid re-entrancy

            foreach (var tunnel in value)
            {
                if (tunnel == null)
                {
                    writer.WriteNullValue();
                    continue;
                }
                // Rely on [JsonDerivedType] attributes for serialization if System.Text.Json supports it well enough.
                // Or explicitly cast and serialize.
                JsonSerializer.Serialize(writer, tunnel, tunnel.GetType(), optionsWithoutThisConverter);
            }
            writer.WriteEndArray();
        }
    }
}
