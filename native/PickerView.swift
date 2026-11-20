import AppKit

final class PickerPanel: NSPanel {
    init(snapshot: FrozenDisplay) {
        super.init(contentRect: snapshot.frame, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
        level = .screenSaver
        isFloatingPanel = true
        isOpaque = true
        backgroundColor = .black
        hasShadow = false
        isMovable = false
        hidesOnDeactivate = false
        acceptsMouseMovedEvents = true
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary, .ignoresCycle]
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }
}

final class PickerView: NSView {
    let snapshot: FrozenDisplay
    weak var owner: SamplerDelegate?
    var selection: PixelPosition?
    private var tracking: NSTrackingArea?
    private let invisibleCursor = NSCursor(image: NSImage(size: NSSize(width: 1, height: 1)), hotSpot: .zero)

    init(snapshot: FrozenDisplay) {
        self.snapshot = snapshot
        super.init(frame: CGRect(origin: .zero, size: snapshot.frame.size))
    }

    required init?(coder: NSCoder) { nil }
    override var isFlipped: Bool { true }
    override var acceptsFirstResponder: Bool { true }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    override func resetCursorRects() { addCursorRect(bounds, cursor: invisibleCursor) }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let tracking { removeTrackingArea(tracking) }
        tracking = NSTrackingArea(rect: bounds, options: [.mouseMoved, .mouseEnteredAndExited, .activeAlways, .inVisibleRect, .cursorUpdate], owner: self)
        if let tracking { addTrackingArea(tracking) }
    }

    override func cursorUpdate(with event: NSEvent) { invisibleCursor.set() }

    override func mouseEntered(with event: NSEvent) {
        invisibleCursor.set()
        owner?.moved(to: self, point: NSEvent.mouseLocation)
    }

    override func mouseMoved(with event: NSEvent) { owner?.moved(to: self, point: NSEvent.mouseLocation) }
    override func mouseDragged(with event: NSEvent) { owner?.moved(to: self, point: NSEvent.mouseLocation) }
    override func mouseDown(with event: NSEvent) { owner?.commit() }
    override func rightMouseDown(with event: NSEvent) { owner?.cancel() }

    override func keyDown(with event: NSEvent) {
        switch event.keyCode {
        case 53: owner?.cancel()
        case 36, 76: owner?.commit()
        case 123: owner?.nudge(columns: -1, rows: 0)
        case 124: owner?.nudge(columns: 1, rows: 0)
        case 125: owner?.nudge(columns: 0, rows: 1)
        case 126: owner?.nudge(columns: 0, rows: -1)
        default: break
        }
    }

    override func draw(_ dirtyRect: NSRect) {
        NSGraphicsContext.current?.imageInterpolation = .none
        NSImage(cgImage: snapshot.image, size: bounds.size).draw(
            in: bounds, from: .zero, operation: .copy, fraction: 1, respectFlipped: true,
            hints: [.interpolation: NSImageInterpolation.none]
        )
        guard let selection else { return }
        let selectedPoint = snapshot.point(at: selection)
        let localPoint = CGPoint(x: selectedPoint.x - snapshot.frame.minX, y: snapshot.frame.maxY - selectedPoint.y)
        drawCrosshair(at: localPoint)
        drawLoupe(selection: selection, at: localPoint)
    }

    private func drawCrosshair(at point: CGPoint) {
        let lines = NSBezierPath()
        for segment in [
            (CGPoint(x: point.x - 11, y: point.y), CGPoint(x: point.x - 4, y: point.y)),
            (CGPoint(x: point.x + 4, y: point.y), CGPoint(x: point.x + 11, y: point.y)),
            (CGPoint(x: point.x, y: point.y - 11), CGPoint(x: point.x, y: point.y - 4)),
            (CGPoint(x: point.x, y: point.y + 4), CGPoint(x: point.x, y: point.y + 11))
        ] {
            lines.move(to: segment.0)
            lines.line(to: segment.1)
        }
        lines.lineCapStyle = .square
        NSColor.black.withAlphaComponent(0.9).setStroke()
        lines.lineWidth = 3
        lines.stroke()
        NSColor.white.setStroke()
        lines.lineWidth = 1
        lines.stroke()
        let center = NSBezierPath(rect: CGRect(x: point.x - 1, y: point.y - 1, width: 2, height: 2))
        NSColor.white.setStroke()
        center.lineWidth = 1
        center.stroke()
    }

    static func loupeRect(near point: CGPoint, in bounds: CGRect) -> CGRect {
        let width: CGFloat = 152
        let height: CGFloat = 204
        var origin = CGPoint(x: point.x + 22, y: point.y + 22)
        if origin.x + width + 8 > bounds.maxX { origin.x = point.x - width - 22 }
        if origin.y + height + 8 > bounds.maxY { origin.y = point.y - height - 22 }
        origin.x = max(bounds.minX + 8, min(origin.x, bounds.maxX - width - 8))
        origin.y = max(bounds.minY + 8, min(origin.y, bounds.maxY - height - 8))
        return CGRect(origin: origin, size: CGSize(width: width, height: height))
    }

    private func drawLoupe(selection: PixelPosition, at point: CGPoint) {
        let card = Self.loupeRect(near: point, in: bounds)
        NSGraphicsContext.saveGraphicsState()
        let shadow = NSShadow()
        shadow.shadowColor = NSColor.black.withAlphaComponent(0.35)
        shadow.shadowBlurRadius = 12
        shadow.shadowOffset = CGSize(width: 0, height: -3)
        shadow.set()
        NSColor(srgbRed: 0.075, green: 0.08, blue: 0.095, alpha: 1).setFill()
        NSBezierPath(roundedRect: card, xRadius: 12, yRadius: 12).fill()
        NSGraphicsContext.restoreGraphicsState()

        let grid = CGRect(x: card.minX + 10, y: card.minY + 10, width: 132, height: 132)
        NSGraphicsContext.saveGraphicsState()
        NSBezierPath(roundedRect: grid, xRadius: 5, yRadius: 5).addClip()
        for row in 0..<11 {
            for column in 0..<11 {
                let pixel = snapshot.pixel(at: PixelPosition(column: selection.column + column - 5, row: selection.row + row - 5))
                pixel.color.setFill()
                CGRect(x: grid.minX + CGFloat(column) * 12, y: grid.minY + CGFloat(row) * 12, width: 12, height: 12).fill()
            }
        }
        let gridLines = NSBezierPath()
        gridLines.lineWidth = 0.5
        for index in 1..<11 {
            gridLines.move(to: CGPoint(x: grid.minX + CGFloat(index) * 12, y: grid.minY))
            gridLines.line(to: CGPoint(x: grid.minX + CGFloat(index) * 12, y: grid.maxY))
            gridLines.move(to: CGPoint(x: grid.minX, y: grid.minY + CGFloat(index) * 12))
            gridLines.line(to: CGPoint(x: grid.maxX, y: grid.minY + CGFloat(index) * 12))
        }
        NSColor.black.withAlphaComponent(0.18).setStroke()
        gridLines.stroke()
        NSGraphicsContext.restoreGraphicsState()

        let center = CGRect(x: grid.minX + 60, y: grid.minY + 60, width: 12, height: 12)
        let blackRing = NSBezierPath(rect: center.insetBy(dx: -1, dy: -1))
        NSColor.black.setStroke()
        blackRing.lineWidth = 3
        blackRing.stroke()
        let whiteRing = NSBezierPath(rect: center)
        NSColor.white.setStroke()
        whiteRing.lineWidth = 1.5
        whiteRing.stroke()

        let value = snapshot.pixel(at: selection)
        value.color.setFill()
        NSBezierPath(roundedRect: CGRect(x: card.minX + 12, y: card.minY + 153, width: 16, height: 16), xRadius: 4, yRadius: 4).fill()
        drawLabel(value.hex, at: CGPoint(x: card.minX + 36, y: card.minY + 151), font: .monospacedSystemFont(ofSize: 13, weight: .medium), color: .white)
        drawLabel("点击取色 · Esc 取消", at: CGPoint(x: card.minX + 12, y: card.minY + 181), font: .systemFont(ofSize: 10), color: NSColor.white.withAlphaComponent(0.55))
    }

    private func drawLabel(_ text: String, at point: CGPoint, font: NSFont, color: NSColor) {
        (text as NSString).draw(at: point, withAttributes: [.font: font, .foregroundColor: color])
    }
}
