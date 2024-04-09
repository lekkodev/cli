import ts from "typescript";
import path from "node:path";

// TODO: We should allow users to specify location
// **/lekko/<namespace>.ts, namespace must be kebab-case alphanumeric
export const LEKKO_FILENAME_REGEX = /lekko\/([a-z][a-z0-9-]*)\.ts$/;

export function isLekkoConfigFile(
  filePath: string,
  configSrcPath: string = "./src/lekko",
) {
  return (
    path.resolve(path.dirname(filePath)) === path.resolve(configSrcPath) &&
    LEKKO_FILENAME_REGEX.test(filePath)
  );
}

export function isObjectType(type: ts.Type): type is ts.ObjectType {
  return (type.flags & ts.TypeFlags.Object) > 0 && "objectFlags" in type;
}

export function isIntrinsicType(type: ts.Type): type is ts.IntrinsicType {
  return (type.flags & ts.TypeFlags.Intrinsic) > 0 && "intrinsicName" in type;
}

export interface CheckedFunctionDeclaration extends ts.FunctionDeclaration {
  name: ts.Identifier;
  body: ts.Block;
}

export function isCheckedFunctionDeclaration(
  node: ts.FunctionDeclaration,
): node is CheckedFunctionDeclaration {
  return node.name !== undefined && node.body !== undefined;
}
