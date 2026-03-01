import Foundation

// MARK: - Models

struct SpectatorSnake {
    var playerId: Int
    var name: String
    var colorIdx: Int
    var alive: Bool
    var isBoosting: Bool
    var score: Int
    var angle: Double
    var segments: [(x: Double, y: Double)]
}

struct SpectatorFood {
    var x: Double
    var y: Double
    var colorIdx: Int
    var radius: Double
}

// MARK: - Connection

class SpectatorConnection: ObservableObject {
    @Published var snakes: [SpectatorSnake] = []
    @Published var foods: [SpectatorFood] = []
    @Published var worldSize: Int = 5000

    private var task: URLSessionWebSocketTask?
    private var targetName: String = ""
    private var metadataCache: [Int: (name: String, colorIdx: Int)] = [:]

    func connect(port: Int, spectateName: String) {
        targetName = spectateName
        guard let url = URL(string: "ws://localhost:\(port)/ws") else { return }
        let session = URLSession(configuration: .default)
        task = session.webSocketTask(with: url)
        task?.resume()
        receiveMessage()
    }

    func disconnect() {
        task?.cancel(with: .normalClosure, reason: nil)
        task = nil
    }

    private func receiveMessage() {
        task?.receive { [weak self] result in
            guard let self = self else { return }
            switch result {
            case .success(let message):
                switch message {
                case .string(let text):
                    self.handleText(text)
                case .data(let data):
                    self.parseBinaryState(data)
                @unknown default:
                    break
                }
                self.receiveMessage()
            case .failure:
                break
            }
        }
    }

    private func handleText(_ text: String) {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let t = json["t"] as? String, t == "welcome" else { return }

        if let ws = json["ws"] as? Int {
            DispatchQueue.main.async { self.worldSize = ws }
        }

        let msg = "{\"t\":\"spectate\",\"name\":\"\(targetName)\"}"
        task?.send(.string(msg)) { _ in }
    }

    // MARK: - Binary Protocol Parser

    private func parseBinaryState(_ data: Data) {
        let bytes = [UInt8](data)
        guard bytes.count >= 4 else { return }
        var o = 0

        let type = bytes[o]; o += 1
        guard type == 1 else { return }
        let flags = bytes[o]; o += 1
        let hasFood = (flags & 1) != 0
        let snakeCount = Int(readUInt16(bytes, &o))

        var parsed: [SpectatorSnake] = []

        for _ in 0..<snakeCount {
            guard o + 2 <= bytes.count else { break }
            let playerId = Int(Int16(bitPattern: readUInt16(bytes, &o)))
            guard o < bytes.count else { break }
            let sFlags = bytes[o]; o += 1
            let alive = (sFlags & 1) != 0
            let isBoosting = (sFlags & 2) != 0
            let hasMeta = (sFlags & 8) != 0

            var name: String
            var colorIdx: Int

            if hasMeta {
                guard o < bytes.count else { break }
                let nameLen = Int(bytes[o]); o += 1
                guard o + nameLen + 1 <= bytes.count else { break }
                name = String(bytes: Array(bytes[o..<o+nameLen]), encoding: .utf8) ?? "Snake"
                o += nameLen
                colorIdx = Int(bytes[o]); o += 1
                metadataCache[playerId] = (name, colorIdx)
            } else if let cached = metadataCache[playerId] {
                name = cached.name
                colorIdx = cached.colorIdx
            } else {
                name = "Snake"
                colorIdx = 0
            }

            guard o + 8 <= bytes.count else { break }
            let score = Int(readUInt16(bytes, &o))
            let angle = Double(Int16(bitPattern: readUInt16(bytes, &o))) / 10000.0
            _ = bytes[o]; o += 1 // boost
            _ = readUInt16(bytes, &o) // targetLen
            _ = bytes[o]; o += 1 // invTimer

            guard o + 2 <= bytes.count else { break }
            let segCount = Int(readUInt16(bytes, &o))
            guard o + segCount * 4 <= bytes.count else { break }

            var sparse: [(x: Double, y: Double)] = []
            for _ in 0..<segCount {
                let x = Double(readUInt16(bytes, &o))
                let y = Double(readUInt16(bytes, &o))
                sparse.append((x, y))
            }

            // Interpolate sparse segments (server sends every 3rd)
            var segs: [(x: Double, y: Double)] = []
            for i in 0..<(sparse.count - 1) {
                segs.append(sparse[i])
                segs.append((
                    sparse[i].x * 2/3 + sparse[i+1].x * 1/3,
                    sparse[i].y * 2/3 + sparse[i+1].y * 1/3
                ))
                segs.append((
                    sparse[i].x * 1/3 + sparse[i+1].x * 2/3,
                    sparse[i].y * 1/3 + sparse[i+1].y * 2/3
                ))
            }
            if !sparse.isEmpty { segs.append(sparse[sparse.count - 1]) }

            parsed.append(SpectatorSnake(
                playerId: playerId, name: name, colorIdx: colorIdx,
                alive: alive, isBoosting: isBoosting, score: score,
                angle: angle, segments: segs
            ))
        }

        // Parse food
        var parsedFood: [SpectatorFood]?
        if hasFood && o + 2 <= bytes.count {
            let foodCount = Int(readUInt16(bytes, &o))
            var fList: [SpectatorFood] = []
            for _ in 0..<foodCount {
                guard o + 7 <= bytes.count else { break }
                let x = Double(readUInt16(bytes, &o))
                let y = Double(readUInt16(bytes, &o))
                let cIdx = Int(bytes[o]); o += 1
                let radius = Double(bytes[o]) / 10.0; o += 1
                _ = bytes[o]; o += 1 // value
                fList.append(SpectatorFood(x: x, y: y, colorIdx: cIdx, radius: radius))
            }
            parsedFood = fList
        }

        DispatchQueue.main.async {
            self.snakes = parsed
            if let f = parsedFood {
                self.foods = f
            }
        }
    }

    private func readUInt16(_ bytes: [UInt8], _ offset: inout Int) -> UInt16 {
        let val = UInt16(bytes[offset]) << 8 | UInt16(bytes[offset + 1])
        offset += 2
        return val
    }
}
