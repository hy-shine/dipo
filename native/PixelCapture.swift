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

struct PixelPosition: Equatable {
    let column: Int
    let row: Int
}

final class FrozenDisplay {
    let frame: CGRect
    let image: CGImage
    let width: Int
    let height: Int
    private let bytes: Data

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
        guard let pointer = context.data, let normalized = context.makeImage() else { throw PickerFailure.bitmap }
        bytes = Data(bytes: pointer, count: width * height * 4)
        self.image = normalized
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

    func pixel(at position: PixelPosition) -> Pixel {
        let column = min(width - 1, max(0, position.column))
        let row = min(height - 1, max(0, position.row))
        let offset = (row * width + column) * 4
        return Pixel(red: bytes[offset], green: bytes[offset + 1], blue: bytes[offset + 2])
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
