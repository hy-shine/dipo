import AppKit

func runPickerChecks(outputDirectory: String?) throws {
    var checks = 0
    func expect(_ condition: @autoclosure () -> Bool, _ message: String) throws {
        guard condition() else { throw NSError(domain: "DipoPickerChecks", code: 1, userInfo: [NSLocalizedDescriptionKey: message]) }
        checks += 1
    }
    func image(width: Int, height: Int, pixel: (Int, Int) -> Pixel) throws -> CGImage {
        var bytes = Data(count: width * height * 4)
        for row in 0..<height {
            for column in 0..<width {
                let value = pixel(column, row)
                let offset = (row * width + column) * 4
                bytes[offset] = value.red
                bytes[offset + 1] = value.green
                bytes[offset + 2] = value.blue
                bytes[offset + 3] = 255
            }
        }
        guard let space = CGColorSpace(name: CGColorSpace.sRGB),
              let provider = CGDataProvider(data: bytes as CFData),
              let result = CGImage(width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: width * 4, space: space, bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.byteOrder32Big.rawValue), provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent)
        else { throw PickerFailure.bitmap }
        return result
    }

    let red = Pixel(red: 255, green: 0, blue: 0)
    let blue = Pixel(red: 0, green: 0, blue: 255)
    let testImage = try image(width: 8, height: 4) { column, row in
        if row == 0 { return red }
        if row == 3 { return blue }
        return Pixel(red: UInt8(column * 20), green: UInt8(row * 30), blue: 27)
    }
    let display = try FrozenDisplay(frame: CGRect(x: -4, y: 6, width: 4, height: 2), image: testImage)
    try expect(display.pixel(at: PixelPosition(column: 0, row: 0)) == red, "top image row must remain red")
    try expect(display.pixel(at: PixelPosition(column: 7, row: 3)) == blue, "bottom image row must remain blue")
    try expect(display.pixel(at: PixelPosition(column: 3, row: 1)) == Pixel(red: 60, green: 30, blue: 27), "sRGB channels must not drift or swap")
    try expect(display.position(at: CGPoint(x: -4, y: 8)) == PixelPosition(column: 0, row: 0), "negative screen origin/top edge")
    try expect(display.position(at: CGPoint(x: 0, y: 6)) == PixelPosition(column: 7, row: 3), "bottom/right bounds clamp")
    try expect(display.position(at: CGPoint(x: -3.25, y: 7.25)) == PixelPosition(column: 1, row: 1), "Retina half-point position")
    for row in 0..<display.height {
        for column in 0..<display.width {
            let position = PixelPosition(column: column, row: row)
            try expect(display.position(at: display.point(at: position)) == position, "point/pixel round trip")
        }
    }
    try expect(display.nudge(PixelPosition(column: 0, row: 0), columns: -1, rows: -1) == PixelPosition(column: 0, row: 0), "nudge clamps at top left")
    try expect(display.nudge(PixelPosition(column: 7, row: 3), columns: 1, rows: 1) == PixelPosition(column: 7, row: 3), "nudge clamps at bottom right")
    try expect(display.nudge(PixelPosition(column: 2, row: 2), columns: 0, rows: -1) == PixelPosition(column: 2, row: 1), "up key moves one pixel up")
    try expect(Pixel(red: 19, green: 22, blue: 27).hex == "#13161B", "HEX serialization")

    let flatImage = try image(width: 16, height: 16) { _, _ in Pixel(red: 19, green: 22, blue: 27) }
    let flat = try FrozenDisplay(frame: CGRect(x: 0, y: 0, width: 8, height: 8), image: flatImage)
    try expect(flat.pixel(at: PixelPosition(column: 8, row: 8)).hex == "#13161B", "known dark sRGB value survives normalization")
    try expect(flat.pixel(at: PixelPosition(column: -5, row: -5)).hex == "#13161B", "loupe pixels clamp outside capture")

    let renderImage = try image(width: 320, height: 240) { _, row in row < 120 ? red : blue }
    let renderDisplay = try FrozenDisplay(frame: CGRect(x: 0, y: 0, width: 320, height: 240), image: renderImage)
    let renderView = PickerView(snapshot: renderDisplay)
    let renderedImage = try renderPickerView(renderView, scale: 1)
    let rendered = try FrozenDisplay(frame: renderDisplay.frame, image: renderedImage)
    try expect(rendered.pixel(at: PixelPosition(column: 10, row: 10)) == red, "frozen screen drawing keeps the top row at the top; got \(rendered.pixel(at: PixelPosition(column: 10, row: 10)).hex)")
    try expect(rendered.pixel(at: PixelPosition(column: 10, row: rendered.height - 10)) == blue, "frozen screen drawing keeps the bottom row at the bottom")

    let bounds = CGRect(x: 0, y: 0, width: 800, height: 500)
    for point in [CGPoint(x: 0, y: 0), CGPoint(x: 799, y: 499), CGPoint(x: 20, y: 480), CGPoint(x: 780, y: 20), CGPoint(x: 400, y: 250)] {
        try expect(bounds.insetBy(dx: 7, dy: 7).contains(PickerView.loupeRect(near: point, in: bounds)), "loupe must stay inside screen")
    }
    if let outputDirectory {
        try FileManager.default.createDirectory(atPath: outputDirectory, withIntermediateDirectories: true)
        let screenshotImage = try image(width: 1600, height: 1000) { column, row in
            if column > 680 && column < 920 && row > 360 && row < 620 {
                return Pixel(red: 19, green: 22, blue: 27)
            }
            return Pixel(red: UInt8(220 + column % 20), green: UInt8(220 + row % 20), blue: 240)
        }
        let snapshot = try FrozenDisplay(frame: bounds, image: screenshotImage)
        let view = PickerView(snapshot: snapshot)
        view.selection = PixelPosition(column: 720, row: 480)
        let bitmap = NSBitmapImageRep(cgImage: try renderPickerView(view, scale: 2))
        guard let png = bitmap.representation(using: .png, properties: [:]) else { throw PickerFailure.bitmap }
        try png.write(to: URL(fileURLWithPath: outputDirectory).appendingPathComponent("picker-native.png"))
        try expect(bitmap.pixelsWide > 0 && bitmap.pixelsHigh > 0, "native rendering produced empty image")
    }
    FileHandle.standardOutput.write(Data("native picker checks passed: \(checks)\n".utf8))
}

private func renderPickerView(_ view: PickerView, scale: CGFloat) throws -> CGImage {
    guard let space = CGColorSpace(name: CGColorSpace.sRGB),
          let context = CGContext(data: nil, width: Int(view.bounds.width * scale), height: Int(view.bounds.height * scale), bitsPerComponent: 8, bytesPerRow: 0, space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.byteOrder32Big.rawValue)
    else { throw PickerFailure.bitmap }
    context.translateBy(x: 0, y: view.bounds.height * scale)
    context.scaleBy(x: scale, y: -scale)
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(cgContext: context, flipped: true)
    view.draw(view.bounds)
    NSGraphicsContext.restoreGraphicsState()
    guard let image = context.makeImage() else { throw PickerFailure.bitmap }
    return image
}
