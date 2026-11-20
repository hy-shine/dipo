import AppKit

private struct CaptureCheckResult: Encodable {
    let status: String
    let samples: [CaptureCheckSample]
}

private struct CaptureCheckSample: Encodable {
    let expected: String
    let sampled: String
    let maximumError: Int
    let scaleX: Double
    let scaleY: Double
}

private final class CaptureCheckView: NSView {
    let colors = [Pixel(red: 255, green: 0, blue: 0), Pixel(red: 0, green: 255, blue: 0), Pixel(red: 0, green: 0, blue: 255), Pixel(red: 19, green: 22, blue: 27)]

    override func draw(_ dirtyRect: NSRect) {
        for index in colors.indices {
            colors[index].color.setFill()
            CGRect(x: CGFloat(index) * bounds.width / CGFloat(colors.count), y: 0, width: bounds.width / CGFloat(colors.count), height: bounds.height).fill()
        }
    }
}

@MainActor
func runCaptureChecks() async throws -> Data {
    guard CGPreflightScreenCaptureAccess() else {
        return try JSONEncoder().encode(CaptureCheckResult(status: "permission", samples: []))
    }
    guard let screen = NSScreen.main ?? NSScreen.screens.first else { throw PickerFailure.noDisplays }
    let width = min(480, screen.visibleFrame.width - 40)
    let height = min(120, screen.visibleFrame.height - 40)
    let frame = CGRect(x: screen.visibleFrame.midX - width / 2, y: screen.visibleFrame.midY - height / 2, width: width, height: height)
    let panel = NSPanel(contentRect: frame, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
    panel.level = .floating
    panel.isOpaque = true
    panel.hasShadow = false
    panel.hidesOnDeactivate = false
    panel.ignoresMouseEvents = true
    panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
    let view = CaptureCheckView(frame: CGRect(origin: .zero, size: frame.size))
    panel.contentView = view
    panel.orderFrontRegardless()
    panel.displayIfNeeded()
    defer { panel.orderOut(nil) }
    try await Task.sleep(for: .milliseconds(200))
    let displays = try await captureDisplays()
    guard let display = displays.first(where: { $0.frame.contains(CGPoint(x: frame.midX, y: frame.midY)) }) else { throw PickerFailure.displayChanged }
    var samples: [CaptureCheckSample] = []
    for index in view.colors.indices {
        let point = CGPoint(x: frame.minX + (CGFloat(index) + 0.5) * frame.width / CGFloat(view.colors.count), y: frame.midY)
        let expected = view.colors[index]
        let sampled = display.pixel(at: display.position(at: point))
        let maximumError = max(abs(Int(expected.red) - Int(sampled.red)), abs(Int(expected.green) - Int(sampled.green)), abs(Int(expected.blue) - Int(sampled.blue)))
        samples.append(CaptureCheckSample(expected: expected.hex, sampled: sampled.hex, maximumError: maximumError, scaleX: Double(display.width) / display.frame.width, scaleY: Double(display.height) / display.frame.height))
    }
    return try JSONEncoder().encode(CaptureCheckResult(status: samples.allSatisfy { $0.maximumError <= 1 } ? "passed" : "mismatch", samples: samples))
}
