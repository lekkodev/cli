import * as lekko from "@lekko/js-sdk";
import * as lekko_pb from "./gen/plugins/config/v1beta1/plugins_pb.ts";
export function getBannerConfig(_ctx, client) {
    var { pathname, } = _ctx;
    try {
        const _config = new lekko_pb.BannerConfig();
        _config.fromBinary(client.getProto("plugins", "banner-config", lekko.ClientContext.fromJSON(_ctx)).value);
        return _config;
    }
    catch (e) {
        if (pathname === "/") {
            return {
                text: "Congratulations, you've successfully configured a banner using Lekko! 🎉",
                cta: {
                    text: "Learn more",
                    url: "https://www.lekko.com/",
                    external: true,
                },
            };
        }
        return {};
    }
}
getBannerConfig._namespaceName = "plugins";
getBannerConfig._configName = "banner-config";
getBannerConfig._evaluationType = "FEATURE_TYPE_PROTO";
export function getBannerStyles(_ctx, client) {
    try {
        const _config = new lekko_pb.BannerStyleConfig();
        _config.fromBinary(client.getProto("plugins", "banner-styles", lekko.ClientContext.fromJSON(_ctx)).value);
        return _config;
    }
    catch (e) {
        return {
            className: "banner-root",
        };
    }
}
getBannerStyles._namespaceName = "plugins";
getBannerStyles._configName = "banner-styles";
getBannerStyles._evaluationType = "FEATURE_TYPE_PROTO";
