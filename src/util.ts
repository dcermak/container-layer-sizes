/**
 * Creates a promise with an added timeout.
 *
 * @param promise  A function that returns the Promise to which a timeout should
 *     be added.
 * @param timeoutMs  The timeout in milliseconds
 * @param errorMsg  Optional error message with which the returned promise is
 *     rejected if the timeout expires.
 */
export function promiseWithTimeout<T>(
  promise: () => Promise<T>,
  timeoutMs: number,
  { errorMsg }: { errorMsg?: string } = {}
): Promise<T> {
  let tmout: NodeJS.Timeout;
  const timeoutPromise = new Promise<never>((_resolve, reject) => {
    tmout = setTimeout(
      () => reject(new Error(errorMsg ?? `Timeout of ${timeoutMs}ms expired`)),
      timeoutMs
    );
  });

  return Promise.race([promise(), timeoutPromise]).then(async (result) => {
    clearTimeout(tmout);
    return result;
  });
}

/** Sleep for the specified number of milliseconds */
export const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

export const formatByte = (bytes: number): string => {
  if (bytes == 0) {
    return "0 Byte";
  }
  const negative = bytes < 0;
  if (negative) {
    bytes *= -1;
  }

  let prefix = Math.floor(Math.log2(bytes) / 10);
  let unitName = new Map([
    [0, "Byte"],
    [1, "KiB"],
    [2, "MiB"],
    [3, "GiB"],
    [4, "TiB"],
    [5, "PiB"]
  ]).get(prefix);

  if (unitName === undefined) {
    throw new Error(
      `${bytes} is too large to be formated, it's more then 2**50`
    );
  }
  const unitValue = 2 ** (10 * prefix);

  return `${negative ? "-" : ""}${(bytes / unitValue).toFixed(2)} ${unitName}`;
};
