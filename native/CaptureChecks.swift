import AppKit

private struct CaptureCheckResult: Encodable {
    let status: String
    /// Whether the display shows Display P3; without it the P3 swatches,
    /// which an sRGB display clips, are left out.
    let wideGamut: Bool
    let samples: [CaptureCheckSample]
}

private struct CaptureCheckSample: Encodable {
    let swatch: String
    /// Extended sRGB components: the swatch as ColorSync converts it, and as
    /// the capture read it.
    let expected: String
    let sampled: String
    /// The largest component difference, in 8-bit steps.
    let maximumError: Int
    let outsideSRGB: Bool
    let scaleX: Double
    let scaleY: Double
}

private struct Swatch {
    let name: String
    let color: NSColor
}

private let srgbSwatches = [
    Swatch(name: "sRGB #FF0000", color: NSColor(srgbRed: 1, green: 0, blue: 0, alpha: 1)),
    Swatch(name: "sRGB #00FF00", color: NSColor(srgbRed: 0, green: 1, blue: 0, alpha: 1)),
    Swatch(name: "sRGB #0000FF", color: NSColor(srgbRed: 0, green: 0, blue: 1, alpha: 1)),
    Swatch(name: "sRGB #13161B", color: Pixel(red: 19, green: 22, blue: 27).color),
]

/// Display P3 colors outside sRGB: the capture must keep them outside.
private let displayP3Swatches = [
    Swatch(name: "Display P3 #00FF00", color: NSColor(displayP3Red: 0, green: 1, blue: 0, alpha: 1)),
    Swatch(name: "Display P3 #FF0000", color: NSColor(displayP3Red: 1, green: 0, blue: 0, alpha: 1)),
    Swatch(name: "Display P3 #FF8000", color: NSColor(displayP3Red: 1, green: 128.0 / 255, blue: 0, alpha: 1)),
]

private final class CaptureCheckView: NSView {
    let swatches: [Swatch]

    init(frame: CGRect, swatches: [Swatch]) {
        self.swatches = swatches
        super.init(frame: frame)
    }

    required init?(coder: NSCoder) { nil }

    override func draw(_ dirtyRect: NSRect) {
        for index in swatches.indices {
            swatches[index].color.setFill()
            CGRect(x: CGFloat(index) * bounds.width / CGFloat(swatches.count), y: 0, width: bounds.width / CGFloat(swatches.count), height: bounds.height).fill()
        }
    }
}

private func components(_ color: ScreenColor) -> String {
    String(format: "%.4f %.4f %.4f", color.red, color.green, color.blue)
}

@MainActor
func runCaptureChecks() async throws -> Data {
    guard let screen = NSScreen.main ?? NSScreen.screens.first else { throw PickerFailure.noDisplays }
    let wideGamut = screen.canRepresent(.p3)
    guard CGPreflightScreenCaptureAccess() else {
        return try JSONEncoder().encode(CaptureCheckResult(status: "permission", wideGamut: wideGamut, samples: []))
    }
    let swatches = srgbSwatches + (wideGamut ? displayP3Swatches : [])
    let width = min(560, screen.visibleFrame.width - 40)
    let height = min(120, screen.visibleFrame.height - 40)
    let frame = CGRect(x: screen.visibleFrame.midX - width / 2, y: screen.visibleFrame.midY - height / 2, width: width, height: height)
    let panel = NSPanel(contentRect: frame, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
    panel.level = .floating
    panel.isOpaque = true
    panel.hasShadow = false
    panel.hidesOnDeactivate = false
    panel.ignoresMouseEvents = true
    panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
    let view = CaptureCheckView(frame: CGRect(origin: .zero, size: frame.size), swatches: swatches)
    panel.contentView = view
    panel.orderFrontRegardless()
    panel.displayIfNeeded()
    defer { panel.orderOut(nil) }
    try await Task.sleep(for: .milliseconds(200))
    let displays = try await captureDisplays()
    guard let display = displays.first(where: { $0.frame.contains(CGPoint(x: frame.midX, y: frame.midY)) }) else { throw PickerFailure.displayChanged }
    var samples: [CaptureCheckSample] = []
    for (index, swatch) in swatches.enumerated() {
        guard let converted = swatch.color.usingColorSpace(.extendedSRGB) else { throw PickerFailure.bitmap }
        let expected = ScreenColor(red: Double(converted.redComponent), green: Double(converted.greenComponent), blue: Double(converted.blueComponent))
        let point = CGPoint(x: frame.minX + (CGFloat(index) + 0.5) * frame.width / CGFloat(swatches.count), y: frame.midY)
        let sampled = try display.color(at: display.position(at: point))
        let difference = max(abs(expected.red - sampled.red), abs(expected.green - sampled.green), abs(expected.blue - sampled.blue))
        samples.append(CaptureCheckSample(
            swatch: swatch.name, expected: components(expected), sampled: components(sampled),
            maximumError: Int((difference * 255).rounded()), outsideSRGB: !sampled.insideSRGB,
            scaleX: Double(display.width) / display.frame.width, scaleY: Double(display.height) / display.frame.height
        ))
    }
    // A swatch passes within one 8-bit step, on the same side of sRGB's edge as the color drawn.
    let passed = zip(samples, swatches).allSatisfy { sample, swatch in
        sample.maximumError <= 1 && sample.outsideSRGB == swatch.name.hasPrefix("Display P3")
    }
    return try JSONEncoder().encode(CaptureCheckResult(status: passed ? "passed" : "mismatch", wideGamut: wideGamut, samples: samples))
}
