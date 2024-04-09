import type ts from "typescript";
import { LEKKO_FILENAME_REGEX } from "./helpers";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Fn = (...args: any) => any;

// type Patch<T extends Record<string, Fn>, M extends keyof T> = (origMethod: T[M]) => (...args: Parameters<T[M]>) => ReturnType<T[M]>;
type Patch<F extends Fn> = (
  origMethod: F,
) => (...args: Parameters<F>) => ReturnType<F>;

function patch<T, M extends keyof T>(
  target: T,
  methodName: M,
  patch: T[M] extends Fn ? Patch<T[M]> : never,
) {
  const method = target[methodName];
  if (typeof method === "function") {
    const origMethod = method.bind(target) as T[M] & Fn;

    target[methodName] = ((...args: Parameters<typeof origMethod>) =>
      // eslint-disable-next-line @typescript-eslint/no-unsafe-return
      patch(origMethod)(...args)) as T[M];
  }
}

export function patchCompilerHost(
  host: ts.CompilerHost,
  sfCache: Map<string, ts.SourceFile>,
) {
  patch(host, "getSourceFile", (origGetSourceFile) => (fileName, ...args) => {
    const cached = sfCache.get(fileName);
    if (cached !== undefined) {
      return cached;
    }
    return origGetSourceFile(fileName, ...args);
  });
}

export function patchProgram(program: ts.Program) {
  patch(
    program,
    "getSemanticDiagnostics",
    (origGetSemanticDiagnostics) =>
      (...args) => {
        const diagnostics = origGetSemanticDiagnostics(...args);
        // Ignore diagnostics on transformed Lekko TS files (e.g. Parameter 'client' implicitly has an 'any' type.)
        return diagnostics.filter(
          (diagnostic) =>
            diagnostic.file?.fileName === undefined ||
            !LEKKO_FILENAME_REGEX.test(diagnostic.file?.fileName),
        );
      },
  );
}
