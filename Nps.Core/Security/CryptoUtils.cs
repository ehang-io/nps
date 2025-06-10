using System;
using System.Security.Cryptography;
using System.Text;

namespace Nps.Core.Security
{
    public static class CryptoUtils
    {
        public static byte[] ToMd5HashBytes(string input)
        {
            if (input == null) throw new ArgumentNullException(nameof(input));

            using (MD5 md5 = MD5.Create())
            {
                byte[] inputBytes = Encoding.UTF8.GetBytes(input);
                byte[] hashBytes = md5.ComputeHash(inputBytes);
                return hashBytes;
            }
        }

        public static string ToMd5Hash(string input)
        {
            if (input == null) throw new ArgumentNullException(nameof(input));

            byte[] hashBytes = ToMd5HashBytes(input);
            StringBuilder sb = new StringBuilder();
            for (int i = 0; i < hashBytes.Length; i++)
            {
                sb.Append(hashBytes[i].ToString("x2")); // Hexadecimal format
            }
            return sb.ToString();
        }
    }
}
