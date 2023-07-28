tpb = proto.package("testproto.v1beta1")

export(
    description = "Configuration for lekko's OAuth 2.0 device authorization process",
    default = tpb.OAuthDeviceConfig(
        verification_uri = "https://app.lekko.com/login/device",
        polling_interval_seconds = 5,
    ),
    rules = [
        ("env == \"staging\"", tpb.OAuthDeviceConfig(
            verification_uri = "https://app-staging.lekko.com/login/device",
            polling_interval_seconds = 5,
        )),
    ],
)
