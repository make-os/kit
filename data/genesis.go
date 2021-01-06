package data

const GenesisData = `[
    {
        "type": "account",
        "address": "os1qfrysysaawvjlgfz5ecqv569adkkw7sxudy36u",
        "balance": "100000000"
    },
    {
        "type": "repo",
        "name": "makeos",
        "helm": true,
        "owners": {
            "os1qfrysysaawvjlgfz5ecqv569adkkw7sxudy36u": {
                "creator": true,
                "joinedAt": 1,
                "veto": true
            }
        },
        "config": {}
    }
]
`
