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
