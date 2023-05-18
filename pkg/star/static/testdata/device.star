tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "Configuration for lekko's OAuth 2.0 device authorization process",
    default = tpb.OAuthDeviceConfig(
        polling_interval_seconds = 5,
        verification_uri = "https://app.lekko.com/login/device",
    ),
    rules = [
        ("env == \"staging\"", tpb.OAuthDeviceConfig(
            polling_interval_seconds = 5,
            verification_uri = "https://app-staging.lekko.com/login/device",
        )),
    ],
)
