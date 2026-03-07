// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "CloudX",
    platforms: [
        .iOS(.v15),
        .macOS(.v12),
    ],
    products: [
        .library(
            name: "CloudX",
            targets: ["CloudX"]
        ),
    ],
    targets: [
        .target(
            name: "CloudX",
            path: "Sources/CloudX"
        ),
        .testTarget(
            name: "CloudXTests",
            dependencies: ["CloudX"],
            path: "Tests/CloudXTests"
        ),
    ]
)
