export interface BannerConfig {
  text?: string;
  cta?: {
    text?: string;
    url?: string;
    external?: boolean;
  };
  permanent?: boolean;
}

export interface BannerStyleConfig {
  className?: string;
  textClassName?: string;
  ctaClassName?: string;
  closeClassName?: string;
}

export function getBannerConfig({
  env,
  pathname,
}: {
  env: string;
  pathname: string;
}): BannerConfig {
  if (env === "development" && pathname === "/404") {
    return {
      cta: {
        text: "Take me back",
        url: "/",
      },
      permanent: true,
      text: "Oh no, page not found",
    };
  } else if (env === "development") {
    return {
      cta: {
        external: true,
        text: "Learn less, do more!!!",
        url: "https://www.lekko.com/",
      },
      text: "Congratulations, you've successfully configured a banner using Lekko! 🎉",
    };
  }
  return {};
}

export function getBannerStyles(): BannerStyleConfig {
  return {
    className: "banner-root",
    closeClassName: "banner-close",
    ctaClassName: "banner-cta",
    textClassName: "banner-text",
  };
}
