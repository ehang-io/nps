using System;
using System.Buffers.Binary;
using System.IO;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace Nps.Core.Protocol.Utils
{
    public static class NetworkHelper
    {
        private const int LengthPrefixSize = 4; // For Int32 length prefix

        /// <summary>
        /// Writes a single byte flag to the stream. Assumes the flag string is a single character.
        /// </summary>
        public static async Task WriteFlagAsync(Stream stream, string flag, CancellationToken cancellationToken = default)
        {
            if (string.IsNullOrEmpty(flag) || flag.Length != 1)
            {
                throw new ArgumentException("Flag must be a single character string.", nameof(flag));
            }
            byte[] flagByte = Encoding.ASCII.GetBytes(flag); // Or UTF8 if flags can be non-ASCII
            await stream.WriteAsync(flagByte, 0, 1, cancellationToken);
        }

        /// <summary>
        /// Reads a single byte flag from the stream and converts it to a string.
        /// </summary>
        public static async Task<string?> ReadFlagAsync(Stream stream, CancellationToken cancellationToken = default)
        {
            byte[] flagByte = new byte[1];
            int bytesRead = await ReadWithTimeoutAsync(stream, flagByte, 0, 1, cancellationToken);
            if (bytesRead == 1)
            {
                return Encoding.ASCII.GetString(flagByte); // Or UTF8
            }
            return null; // Or throw exception if flag is mandatory
        }

        /// <summary>
        /// Writes a byte array to the stream, prefixed by its length (Int32, LittleEndian).
        /// </summary>
        public static async Task WriteBytesWithLengthPrefixAsync(Stream stream, byte[] data, CancellationToken cancellationToken = default)
        {
            if (data == null) throw new ArgumentNullException(nameof(data));

            byte[] lengthBytes = new byte[LengthPrefixSize];
            BinaryPrimitives.WriteInt32LittleEndian(lengthBytes, data.Length);

            await stream.WriteAsync(lengthBytes, 0, LengthPrefixSize, cancellationToken);
            await stream.WriteAsync(data, 0, data.Length, cancellationToken);
        }

        /// <summary>
        /// Reads a byte array from the stream that is prefixed by its length (Int32, LittleEndian).
        /// </summary>
        public static async Task<byte[]?> ReadBytesWithLengthPrefixAsync(Stream stream, CancellationToken cancellationToken = default)
        {
            byte[] lengthBytes = new byte[LengthPrefixSize];
            int bytesRead = await ReadWithTimeoutAsync(stream, lengthBytes, 0, LengthPrefixSize, cancellationToken);

            if (bytesRead < LengthPrefixSize)
            {
                // Not enough data for length prefix, could be connection closed or timeout
                return null;
            }

            int dataLength = BinaryPrimitives.ReadInt32LittleEndian(lengthBytes);
            if (dataLength < 0) // Or some reasonable max length check
            {
                throw new IOException("Invalid data length received.");
            }
            if (dataLength == 0)
            {
                return Array.Empty<byte>();
            }

            byte[] data = new byte[dataLength];
            bytesRead = await ReadWithTimeoutAsync(stream, data, 0, dataLength, cancellationToken);

            if (bytesRead < dataLength)
            {
                // Not enough data received, could be connection closed or timeout
                // Consider how to handle partial reads if that's a valid scenario.
                // For now, assuming full message or error.
                throw new EndOfStreamException($"Expected {dataLength} bytes but only received {bytesRead}.");
            }
            return data;
        }

        /// <summary>
        /// Writes a string to the stream, UTF-8 encoded, prefixed by its byte length.
        /// </summary>
        public static async Task WriteStringWithLengthPrefixAsync(Stream stream, string text, CancellationToken cancellationToken = default)
        {
            if (text == null) throw new ArgumentNullException(nameof(text));
            byte[] data = Encoding.UTF8.GetBytes(text);
            await WriteBytesWithLengthPrefixAsync(stream, data, cancellationToken);
        }

        /// <summary>
        /// Reads a UTF-8 encoded string from the stream that was prefixed by its byte length.
        /// </summary>
        public static async Task<string?> ReadStringWithLengthPrefixAsync(Stream stream, CancellationToken cancellationToken = default)
        {
            byte[]? data = await ReadBytesWithLengthPrefixAsync(stream, cancellationToken);
            return data != null ? Encoding.UTF8.GetString(data) : null;
        }

        /// <summary>
        /// Writes the provided byte array directly to the stream.
        /// </summary>
        public static async Task WriteFixedLengthBytesAsync(Stream stream, byte[] data, CancellationToken cancellationToken = default)
        {
            if (data == null) throw new ArgumentNullException(nameof(data));
            await stream.WriteAsync(data, 0, data.Length, cancellationToken);
        }

        /// <summary>
        /// Reads a fixed number of bytes from the stream.
        /// </summary>
        /// <exception cref="EndOfStreamException">Thrown if the end of the stream is reached before reading the specified number of bytes.</exception>
        public static async Task<byte[]> ReadFixedLengthBytesAsync(Stream stream, int length, CancellationToken cancellationToken = default)
        {
            if (length < 0) throw new ArgumentOutOfRangeException(nameof(length));
            if (length == 0) return Array.Empty<byte>();

            byte[] buffer = new byte[length];
            int totalBytesRead = 0;
            while (totalBytesRead < length)
            {
                int bytesRead = await ReadWithTimeoutAsync(stream, buffer, totalBytesRead, length - totalBytesRead, cancellationToken);
                if (bytesRead == 0) // Stream closed or timeout occurred before any byte was read in this call
                {
                    throw new EndOfStreamException($"Connection closed prematurely. Expected {length} bytes but only received {totalBytesRead}.");
                }
                totalBytesRead += bytesRead;
            }
            return buffer;
        }

        /// <summary>
        /// Helper method to read from a Stream with a timeout if CancellationToken is used.
        /// </summary>
        private static async Task<int> ReadWithTimeoutAsync(Stream stream, byte[] buffer, int offset, int count, CancellationToken cancellationToken)
        {
            // If cancellationToken is default or None, just call ReadAsync
            if (cancellationToken == default || cancellationToken == CancellationToken.None)
            {
                return await stream.ReadAsync(buffer, offset, count, cancellationToken);
            }

            // Create a task that completes when ReadAsync completes or when cancellation is requested
            var readTask = stream.ReadAsync(buffer, offset, count, cancellationToken);

            // Awaiting the task will throw OperationCanceledException if cancellationToken is triggered during the read.
            return await readTask;
        }

        /// <summary>
        /// Writes the predefined CONN_TEST byte sequence to the stream.
        /// </summary>
        public static async Task WriteConnTestAsync(Stream stream, CancellationToken cancellationToken = default)
        {
            await stream.WriteAsync(ProtocolConstants.ConnTestBytes, 0, ProtocolConstants.ConnTestBytes.Length, cancellationToken);
        }

        /// <summary>
        /// Reads a sequence of bytes from the stream and verifies if it matches ProtocolConstants.ConnTestBytes.
        /// </summary>
        /// <returns>True if the sequence matches, false otherwise.</returns>
        public static async Task<bool> VerifyConnTestAsync(Stream stream, CancellationToken cancellationToken = default)
        {
            byte[] buffer = new byte[ProtocolConstants.ConnTestBytes.Length];
            try
            {
                int bytesRead = await ReadWithTimeoutAsync(stream, buffer, 0, buffer.Length, cancellationToken);
                if (bytesRead < buffer.Length)
                {
                    return false; // Not enough bytes read to match
                }

                for (int i = 0; i < buffer.Length; i++)
                {
                    if (buffer[i] != ProtocolConstants.ConnTestBytes[i])
                    {
                        return false; // Byte sequence mismatch
                    }
                }
                return true; // Sequence matches
            }
            catch (IOException)
            {
                // Handle stream read errors (e.g., connection closed) as verification failure
                return false;
            }
            catch (OperationCanceledException)
            {
                // If operation was cancelled, verification didn't complete
                return false;
            }
        }
    }
}
