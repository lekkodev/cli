import * as fs from "fs";
import { spawnSync } from "child_process";
import { type TransformerTarget } from "./types";

interface NodeLekkoVars {
  LEKKO_API_KEY: string;
  LEKKO_REPO_NAME: string;
}

interface ViteLekkoVars {
  VITE_LEKKO_API_KEY: string;
  VITE_LEKKO_REPOSITORY_OWNER: string;
  VITE_LEKKO_REPOSITORY_NAME: string;
}

interface NextLekkoVars {
  NEXT_PUBLIC_LEKKO_API_KEY: string;
  NEXT_PUBLIC_LEKKO_REPOSITORY_OWNER: string;
  NEXT_PUBLIC_LEKKO_REPOSITORY_NAME: string;
}

// TODO: Classify error types like missing CLI, missing API key, etc.
function getVarsFromCLI(
  target: TransformerTarget,
): NodeLekkoVars | ViteLekkoVars | NextLekkoVars {
  const apiKeyCmd = spawnSync("lekko", ["apikey", "show"], {
    encoding: "utf-8",
  });
  if (apiKeyCmd.error !== undefined || apiKeyCmd.status !== 0) {
    // TODO: Differentiate between missing or other reasons
    throw new Error("failed to read API key");
  }
  const apiKey = apiKeyCmd.stdout.trim();

  const repoCmd = spawnSync("lekko", ["repo", "remote"], { encoding: "utf-8" });
  if (repoCmd.error !== undefined || repoCmd.status !== 0) {
    // TODO: Differentiate between missing or other reasons
    throw new Error("failed to read repo name");
  }
  const repoName = repoCmd.stdout.trim();

  switch (target) {
    case "node": {
      return {
        LEKKO_API_KEY: apiKey,
        LEKKO_REPO_NAME: repoName,
      };
    }
    case "vite": {
      const [owner, name] = repoName.split("/");
      return {
        VITE_LEKKO_API_KEY: apiKey,
        VITE_LEKKO_REPOSITORY_OWNER: owner,
        VITE_LEKKO_REPOSITORY_NAME: name,
      };
    }
    case "next": {
      const [owner, name] = repoName.split("/");
      return {
        NEXT_PUBLIC_LEKKO_API_KEY: apiKey,
        NEXT_PUBLIC_LEKKO_REPOSITORY_OWNER: owner,
        NEXT_PUBLIC_LEKKO_REPOSITORY_NAME: name,
      };
    }
  }
}

// For now, assumes that all env var files will be located at project root.
// This assumption might be challenged at some point.
// TODO: support CRA env vars
export function emitEnvVars(
  target: TransformerTarget,
  filename: string = ".env",
) {
  let contents = "";
  try {
    contents = fs.readFileSync(filename, {
      encoding: "utf-8",
    });
  } catch (e) {
    // File not found, probably, which is fine - we'll just write a new one
  }

  let lekkoVars;
  try {
    lekkoVars = getVarsFromCLI(target);
  } catch (e) {
    throw new Error(
      "Failed to emit Lekko environment variables: please check that the Lekko CLI is installed and an authorized user is logged in.",
    );
  }

  // Regex-based search & replace for now
  Object.entries(lekkoVars).forEach(([key, value]) => {
    // Find based on key, replace only value
    const pattern = new RegExp(
      `^(?<prefix>${key}[ \t]*=[ \t]*)["']?(?<value>[a-zA-Z0-9_-]*)["']?$`,
      "m",
    );
    if (pattern.test(contents)) {
      contents = contents.replace(pattern, `${key}=${value}`);
    } else {
      // If not in file, append
      contents += `${key}=${value}\n`;
    }
  });
  // Write back
  fs.writeFileSync(filename, contents);

  return lekkoVars;
}
