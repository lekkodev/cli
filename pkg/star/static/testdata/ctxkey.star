export(
    Config(
        description = "test config for context keys",
        default = "foo",
        rules = [
            ("user.email == \"asdf@gmail.com\"", "bar"),
        ],
    ),
)
