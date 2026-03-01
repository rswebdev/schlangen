import SwiftUI

// MARK: - Color Palettes

private let snakeColors: [(h: Color, b: Color)] = [
    (Color(red: 1.0, green: 0.267, blue: 0.4),   Color(red: 0.8, green: 0.133, blue: 0.267)),   // #ff4466 / #cc2244
    (Color(red: 0.267, green: 0.733, blue: 1.0),  Color(red: 0.133, green: 0.6, blue: 0.867)),   // #44bbff / #2299dd
    (Color(red: 0.267, green: 1.0, blue: 0.533),  Color(red: 0.133, green: 0.8, blue: 0.4)),     // #44ff88 / #22cc66
    (Color(red: 1.0, green: 0.667, blue: 0.133),  Color(red: 0.867, green: 0.533, blue: 0.0)),   // #ffaa22 / #dd8800
    (Color(red: 1.0, green: 0.4, blue: 1.0),      Color(red: 0.8, green: 0.267, blue: 0.8)),     // #ff66ff / #cc44cc
    (Color(red: 1.0, green: 1.0, blue: 0.267),    Color(red: 0.8, green: 0.8, blue: 0.133)),     // #ffff44 / #cccc22
    (Color(red: 1.0, green: 0.533, blue: 0.267),  Color(red: 0.8, green: 0.4, blue: 0.133)),     // #ff8844 / #cc6622
    (Color(red: 0.533, green: 1.0, blue: 1.0),    Color(red: 0.4, green: 0.8, blue: 0.8)),       // #88ffff / #66cccc
    (Color(red: 0.667, green: 0.533, blue: 1.0),  Color(red: 0.533, green: 0.4, blue: 0.8)),     // #aa88ff / #8866cc
    (Color(red: 1.0, green: 0.533, blue: 0.667),  Color(red: 0.8, green: 0.4, blue: 0.533)),     // #ff88aa / #cc6688
    (Color(red: 0.533, green: 1.0, blue: 0.267),  Color(red: 0.4, green: 0.8, blue: 0.133)),     // #88ff44 / #66cc22
    (Color(red: 0.267, green: 1.0, blue: 0.8),    Color(red: 0.133, green: 0.8, blue: 0.667)),   // #44ffcc / #22ccaa
]

private let foodColors: [Color] = [
    Color(red: 1.0, green: 0.42, blue: 0.42),     // #ff6b6b
    Color(red: 0.933, green: 0.353, blue: 0.141),  // #ee5a24
    Color(red: 1.0, green: 0.827, blue: 0.165),    // #ffd32a
    Color(red: 0.043, green: 0.91, blue: 0.506),   // #0be881
    Color(red: 0.094, green: 0.863, blue: 1.0),    // #18dcff
    Color(red: 0.443, green: 0.345, blue: 0.886),  // #7158e2
    Color(red: 1.0, green: 0.22, blue: 0.22),      // #ff3838
    Color(red: 0.227, green: 0.89, blue: 0.455),   // #3ae374
    Color(red: 1.0, green: 0.624, blue: 0.263),    // #ff9f43
    Color(red: 0.647, green: 0.369, blue: 0.918),  // #a55eea
    Color(red: 1.0, green: 0.388, blue: 0.282),    // #ff6348
    Color(red: 0.18, green: 0.835, blue: 0.451),   // #2ed573
]

// MARK: - Constants

private let gridSpacing: Double = 60
private let boundaryMargin: Double = 50
private let headRadiusBase: Double = 12
private let bodyRadiusBase: Double = 10
private let bgColor = Color(red: 0.04, green: 0.04, blue: 0.18) // #0a0a2e

// MARK: - SpectatorView

struct SpectatorView: View {
    let port: Int
    let spectateName: String

    @StateObject private var conn = SpectatorConnection()
    @State private var cameraX: Double = 0
    @State private var cameraY: Double = 0

    var body: some View {
        ZStack {
            bgColor.ignoresSafeArea()

            TimelineView(.animation) { timeline in
                Canvas { ctx, size in
                    let snakes = conn.snakes
                    let foods = conn.foods
                    let ws = Double(conn.worldSize)

                    // Update camera
                    let cam = computeCamera(snakes: snakes, size: size, ws: ws)

                    drawGrid(ctx: &ctx, size: size, cam: cam, ws: ws)
                    drawBoundary(ctx: &ctx, size: size, cam: cam, ws: ws)
                    drawFoodItems(ctx: &ctx, size: size, cam: cam, foods: foods)
                    for snake in snakes where snake.alive {
                        drawSnake(ctx: &ctx, size: size, cam: cam, snake: snake)
                    }
                }
            }

            // Overlay
            VStack {
                Text("Spectating: \(spectateName)")
                    .font(.caption)
                    .fontWeight(.semibold)
                    .foregroundColor(.white.opacity(0.7))
                    .padding(.horizontal, 20)
                    .padding(.vertical, 8)
                    .background(Color.black.opacity(0.4))
                    .cornerRadius(8)
                    .padding(.top, 50)
                Spacer()
            }
        }
        .onAppear { conn.connect(port: port, spectateName: spectateName) }
        .onDisappear { conn.disconnect() }
    }

    // MARK: - Camera

    private func computeCamera(snakes: [SpectatorSnake], size: CGSize, ws: Double) -> (x: Double, y: Double) {
        var targetX = ws / 2
        var targetY = ws / 2

        // Find the spectated snake
        if let target = snakes.first(where: { $0.name == spectateName && $0.alive && !$0.segments.isEmpty }) {
            targetX = target.segments[0].x
            targetY = target.segments[0].y
        } else if let top = snakes.filter({ $0.alive && !$0.segments.isEmpty }).max(by: { $0.score < $1.score }) {
            // Fallback: follow highest-score snake
            targetX = top.segments[0].x
            targetY = top.segments[0].y
        }

        let goalX = targetX - Double(size.width) / 2
        let goalY = targetY - Double(size.height) / 2

        cameraX += (goalX - cameraX) * 0.1
        cameraY += (goalY - cameraY) * 0.1

        return (cameraX, cameraY)
    }

    // MARK: - Grid

    private func drawGrid(ctx: inout GraphicsContext, size: CGSize, cam: (x: Double, y: Double), ws: Double) {
        let startX = max(0, cam.x - 10)
        let endX = min(ws, cam.x + Double(size.width) + 10)
        let startY = max(0, cam.y - 10)
        let endY = min(ws, cam.y + Double(size.height) + 10)

        let gridColor = Color.white.opacity(0.04)

        var gridPath = Path()

        // Vertical lines
        var x = (startX / gridSpacing).rounded(.down) * gridSpacing
        while x <= endX {
            let sx = x - cam.x
            gridPath.move(to: CGPoint(x: sx, y: max(0, startY - cam.y)))
            gridPath.addLine(to: CGPoint(x: sx, y: min(Double(size.height), endY - cam.y)))
            x += gridSpacing
        }

        // Horizontal lines
        var y = (startY / gridSpacing).rounded(.down) * gridSpacing
        while y <= endY {
            let sy = y - cam.y
            gridPath.move(to: CGPoint(x: max(0, startX - cam.x), y: sy))
            gridPath.addLine(to: CGPoint(x: min(Double(size.width), endX - cam.x), y: sy))
            y += gridSpacing
        }

        ctx.stroke(gridPath, with: .color(gridColor), lineWidth: 1)
    }

    // MARK: - Boundary

    private func drawBoundary(ctx: inout GraphicsContext, size: CGSize, cam: (x: Double, y: Double), ws: Double) {
        let bx = boundaryMargin - cam.x
        let by = boundaryMargin - cam.y
        let bw = ws - boundaryMargin * 2
        let bh = ws - boundaryMargin * 2

        let rect = CGRect(x: bx, y: by, width: bw, height: bh)
        let borderColor = Color.red.opacity(0.4)

        ctx.stroke(
            Path(rect),
            with: .color(borderColor),
            style: StrokeStyle(lineWidth: 2, dash: [12, 8])
        )
    }

    // MARK: - Food

    private func drawFoodItems(ctx: inout GraphicsContext, size: CGSize, cam: (x: Double, y: Double), foods: [SpectatorFood]) {
        let w = Double(size.width)
        let h = Double(size.height)

        for f in foods {
            let sx = f.x - cam.x
            let sy = f.y - cam.y

            // Viewport culling
            guard sx > -20 && sx < w + 20 && sy > -20 && sy < h + 20 else { continue }

            let r = f.radius
            let colorIdx = f.colorIdx % foodColors.count
            let color = foodColors[colorIdx]

            // Glow
            let glowRect = CGRect(x: sx - r - 4, y: sy - r - 4, width: (r + 4) * 2, height: (r + 4) * 2)
            ctx.fill(Path(ellipseIn: glowRect), with: .color(color.opacity(0.2)))

            // Main circle
            let mainRect = CGRect(x: sx - r, y: sy - r, width: r * 2, height: r * 2)
            ctx.fill(Path(ellipseIn: mainRect), with: .color(color))
        }
    }

    // MARK: - Snake

    private func drawSnake(ctx: inout GraphicsContext, size: CGSize, cam: (x: Double, y: Double), snake: SpectatorSnake) {
        let segs = snake.segments
        guard segs.count >= 2 else { return }

        let colorIdx = snake.colorIdx % snakeColors.count
        let hColor = snakeColors[colorIdx].h
        let bColor = snakeColors[colorIdx].b
        let segCount = Double(segs.count)

        let headR = headRadiusBase + min(segCount * 0.03, 6)
        let bodyR = bodyRadiusBase + min(segCount * 0.025, 5)

        let w = Double(size.width)
        let h = Double(size.height)

        // Body segments (tail to head)
        for i in stride(from: segs.count - 1, through: 1, by: -1) {
            let sx = segs[i].x - cam.x
            let sy = segs[i].y - cam.y

            // Viewport culling
            guard sx > -30 && sx < w + 30 && sy > -30 && sy < h + 30 else { continue }

            let r = bodyR * (1.0 - (Double(i) / segCount) * 0.3)
            let color = (i / 3) % 2 == 0 ? hColor : bColor

            let rect = CGRect(x: sx - r, y: sy - r, width: r * 2, height: r * 2)
            ctx.fill(Path(ellipseIn: rect), with: .color(color))
        }

        // Head
        let hx = segs[0].x - cam.x
        let hy = segs[0].y - cam.y

        // Glow ring
        let glowR = headR + 5
        let glowRect = CGRect(x: hx - glowR, y: hy - glowR, width: glowR * 2, height: glowR * 2)
        ctx.fill(Path(ellipseIn: glowRect), with: .color(hColor.opacity(0.25)))

        // Head circle
        let headRect = CGRect(x: hx - headR, y: hy - headR, width: headR * 2, height: headR * 2)
        ctx.fill(Path(ellipseIn: headRect), with: .color(hColor))
        ctx.stroke(Path(ellipseIn: headRect), with: .color(bColor), lineWidth: 2)

        // Eyes
        let eo = headR * 0.45
        let er = headR * 0.3
        let pr = er * 0.55
        let angle = snake.angle

        for s in stride(from: -1.0, through: 1.0, by: 2.0) {
            let ex = hx + cos(angle - s * 0.5) * eo
            let ey = hy + sin(angle - s * 0.5) * eo

            let eyeRect = CGRect(x: ex - er, y: ey - er, width: er * 2, height: er * 2)
            ctx.fill(Path(ellipseIn: eyeRect), with: .color(.white))

            let px = ex + cos(angle) * pr * 0.4
            let py = ey + sin(angle) * pr * 0.4
            let pupilRect = CGRect(x: px - pr, y: py - pr, width: pr * 2, height: pr * 2)
            ctx.fill(Path(ellipseIn: pupilRect), with: .color(Color(red: 0.067, green: 0.067, blue: 0.067)))
        }

        // Name label
        let nameText = Text(snake.name)
            .font(.system(size: 13, weight: .bold))
            .foregroundColor(.white.opacity(0.8))
        ctx.draw(ctx.resolve(nameText), at: CGPoint(x: hx, y: hy - headR - 14), anchor: .center)

        // Length label
        let lenText = Text("\(segs.count)")
            .font(.system(size: 10))
            .foregroundColor(.white.opacity(0.4))
        ctx.draw(ctx.resolve(lenText), at: CGPoint(x: hx, y: hy - headR - 3), anchor: .center)
    }
}
