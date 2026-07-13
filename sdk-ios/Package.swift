// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "VectorSpace",
    platforms: [
        .iOS(.v15),
        .macOS(.v12),
    ],
    products: [
        .library(
            name: "VectorSpace",
            targets: ["VectorSpace"]
        ),
    ],
    targets: [
        .target(
            name: "VectorSpace",
            path: "Sources/VectorSpace"
        ),
        .testTarget(
            name: "VectorSpaceTests",
            dependencies: ["VectorSpace"],
            path: "Tests/VectorSpaceTests"
        ),
    ]
)
