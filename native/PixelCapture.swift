import AppKit
import CoreMedia
import ScreenCaptureKit

enum PickerFailure: Error {
    case bitmap
    case noDisplays
    case displayChanged
}

struct Pixel: Equatable {
    let red: UInt8
    let green: UInt8
    let blue: UInt8

    var color: NSColor {
        NSColor(srgbRed: CGFloat(red) / 255, green: CGFloat(green) / 255, blue: CGFloat(blue) / 255, alpha: 1)
    }

    var hex: String { String(format: "#%02X%02X%02X", red, green, blue) }
}

/// A color in extended sRGB: sRGB's primaries and transfer curve, with
/// components that may pass 0 and 1, so a color that sRGB cannot show keeps
/// how far outside it lies.
struct ScreenColor: Equatable {
    let red: Double
    let green: Double
    let blue: Double

    /// The nearest 8-bit sRGB color: each component clamped to sRGB and rounded.
    var pixel: Pixel { Pixel(red: Self.channel(red), green: Self.channel(green), blue: Self.channel(blue)) }

    /// Whether sRGB can show the color: every component rounds to an 8-bit
    /// value without clamping. Go's colour package draws the same line.
    var insideSRGB: Bool { [red, green, blue].allSatisfy { $0 * 255 >= -0.5 && $0 * 255 <= 255.5 } }

    /// The color to draw, outside sRGB included.
    var color: NSColor {
        NSColor(colorSpace: .extendedSRGB, components: [CGFloat(red), CGFloat(green), CGFloat(blue), 1], count: 4)
    }

    private static func channel(_ value: Double) -> UInt8 { UInt8((min(1, max(0, value)) * 255).rounded()) }
}

struct PixelPosition: Equatable {
    let column: Int
    let row: Int
}

final class FrozenDisplay {
    let frame: CGRect
    /// The screen in 8-bit sRGB, to draw it frozen.
    let image: CGImage
    let width: Int
    let height: Int
    /// The screen as captured, which colors are read from: converting only the
    /// pixels read keeps colors outside sRGB without holding a second
    /// full-screen copy in floating point.
    private let capture: CGImage

    init(frame: CGRect, image: CGImage) throws {
        width = image.width
        height = image.height
        self.frame = frame
        guard width > 0, height > 0, frame.width > 0, frame.height > 0,
              let space = CGColorSpace(name: CGColorSpace.sRGB),
              let context = CGContext(
                data: nil, width: width, height: height, bitsPerComponent: 8,
                bytesPerRow: width * 4, space: space,
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.byteOrder32Big.rawValue
              )
        else { throw PickerFailure.bitmap }
        context.interpolationQuality = .none
        context.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
        guard let normalized = context.makeImage() else { throw PickerFailure.bitmap }
        self.image = normalized
        capture = image
    }

    func position(at point: CGPoint) -> PixelPosition {
        PixelPosition(
            column: min(width - 1, max(0, Int(floor((point.x - frame.minX) / frame.width * CGFloat(width))))),
            row: min(height - 1, max(0, Int(floor((frame.maxY - point.y) / frame.height * CGFloat(height)))))
        )
    }

    func point(at position: PixelPosition) -> CGPoint {
        CGPoint(
            x: frame.minX + (CGFloat(position.column) + 0.5) / CGFloat(width) * frame.width,
            y: frame.maxY - (CGFloat(position.row) + 0.5) / CGFloat(height) * frame.height
        )
    }

    func color(at position: PixelPosition) throws -> ScreenColor {
        try colors(around: position, radius: 0)[0]
    }

    /// The colors of the square of pixels radius pixels around center, row by
    /// row from the top left; a position past the capture's edge reads the
    /// nearest pixel on it.
    func colors(around center: PixelPosition, radius: Int) throws -> [ScreenColor] {
        let columns = max(0, min(width - 1, center.column - radius))...max(0, min(width - 1, center.column + radius))
        let rows = max(0, min(height - 1, center.row - radius))...max(0, min(height - 1, center.row + radius))
        guard let region = capture.cropping(to: CGRect(x: columns.lowerBound, y: rows.lowerBound, width: columns.count, height: rows.count)),
              let space = CGColorSpace(name: CGColorSpace.extendedSRGB)
        else { throw PickerFailure.bitmap }
        var components = [Float](repeating: 0, count: columns.count * rows.count * 4)
        let drawn = components.withUnsafeMutableBytes { buffer in
            guard let context = CGContext(
                data: buffer.baseAddress, width: columns.count, height: rows.count, bitsPerComponent: 32,
                bytesPerRow: columns.count * 16, space: space,
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.floatComponents.rawValue | CGBitmapInfo.byteOrder32Little.rawValue
            ) else { return false }
            context.interpolationQuality = .none
            context.setBlendMode(.copy)
            context.draw(region, in: CGRect(x: 0, y: 0, width: columns.count, height: rows.count))
            return true
        }
        guard drawn else { throw PickerFailure.bitmap }
        var result: [ScreenColor] = []
        result.reserveCapacity((2 * radius + 1) * (2 * radius + 1))
        for row in center.row - radius...center.row + radius {
            for column in center.column - radius...center.column + radius {
                let offset = ((min(rows.upperBound, max(rows.lowerBound, row)) - rows.lowerBound) * columns.count
                    + min(columns.upperBound, max(columns.lowerBound, column)) - columns.lowerBound) * 4
                result.append(ScreenColor(red: Double(components[offset]), green: Double(components[offset + 1]), blue: Double(components[offset + 2])))
            }
        }
        return result
    }

    func nudge(_ position: PixelPosition, columns: Int, rows: Int) -> PixelPosition {
        PixelPosition(
            column: min(width - 1, max(0, position.column + columns)),
            row: min(height - 1, max(0, position.row + rows))
        )
    }
}

@MainActor
func captureDisplays() async throws -> [FrozenDisplay] {
    let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
    let screens = NSScreen.screens
    var snapshots: [FrozenDisplay] = []
    for screen in screens {
        guard let identifier = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber,
              let display = content.displays.first(where: { $0.displayID == identifier.uint32Value })
        else { throw PickerFailure.displayChanged }
        let configuration = SCStreamConfiguration()
        configuration.width = max(1, Int((screen.frame.width * screen.backingScaleFactor).rounded()))
        configuration.height = max(1, Int((screen.frame.height * screen.backingScaleFactor).rounded()))
        configuration.showsCursor = false
        let filter = SCContentFilter(display: display, excludingWindows: [])
        if #available(macOS 14.2, *) { filter.includeMenuBar = true }
        let image = try await captureImage(filter: filter, configuration: configuration)
        snapshots.append(try FrozenDisplay(frame: screen.frame, image: image))
    }
    guard !snapshots.isEmpty else { throw PickerFailure.noDisplays }
    guard NSScreen.screens.map(\.frame) == screens.map(\.frame) else { throw PickerFailure.displayChanged }
    return snapshots
}

private func captureImage(filter: SCContentFilter, configuration: SCStreamConfiguration) async throws -> CGImage {
    configuration.pixelFormat = kCVPixelFormatType_64RGBAHalf
    configuration.colorSpaceName = CGColorSpace.extendedLinearDisplayP3
    if #available(macOS 15.0, *) { configuration.captureDynamicRange = .hdrLocalDisplay }
    let sample = try await SCScreenshotManager.captureSampleBuffer(contentFilter: filter, configuration: configuration)
    guard let buffer = sample.imageBuffer,
          CVPixelBufferGetPixelFormatType(buffer) == kCVPixelFormatType_64RGBAHalf,
          let space = CGColorSpace(name: CGColorSpace.extendedLinearDisplayP3)
    else { throw PickerFailure.bitmap }
    CVPixelBufferLockBaseAddress(buffer, .readOnly)
    defer { CVPixelBufferUnlockBaseAddress(buffer, .readOnly) }
    guard let base = CVPixelBufferGetBaseAddress(buffer) else { throw PickerFailure.bitmap }
    let height = CVPixelBufferGetHeight(buffer)
    let bytesPerRow = CVPixelBufferGetBytesPerRow(buffer)
    let data = Data(bytes: base, count: bytesPerRow * height)
    guard let provider = CGDataProvider(data: data as CFData),
          let image = CGImage(
            width: CVPixelBufferGetWidth(buffer), height: height,
            bitsPerComponent: 16, bitsPerPixel: 64, bytesPerRow: bytesPerRow,
            space: space,
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.floatComponents.rawValue | CGBitmapInfo.byteOrder16Little.rawValue),
            provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent
          )
    else { throw PickerFailure.bitmap }
    return image
}
