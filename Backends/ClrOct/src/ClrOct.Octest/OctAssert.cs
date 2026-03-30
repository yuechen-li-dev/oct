namespace ClrOct.Octest;

public static class OctAssert
{
    public static void True(bool condition, string? message = null)
    {
        if (!condition)
        {
            throw new OctAssertException(message ?? "Expected condition to be true.");
        }
    }

    public static void Equal<T>(T expected, T actual, string? message = null)
    {
        if (!EqualityComparer<T>.Default.Equals(expected, actual))
        {
            var defaultMessage = $"Expected <{expected}> but was <{actual}>.";
            throw new OctAssertException(message ?? defaultMessage);
        }
    }
}

public sealed class OctAssertException(string message) : Exception(message);
