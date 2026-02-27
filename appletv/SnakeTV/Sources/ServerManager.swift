import SwiftUI
import Mobile

class ServerManager: ObservableObject {
    @Published var isRunning = false
    @Published var connectURL = ""
    @Published var localIP = ""
    @Published var errorMessage: String?
    @Published var stats: GameStats = GameStats()

    let port: Int = 8080
    private var statsTimer: Timer?

    func startServer(with rules: HouseRules = .defaults) {
        let jsonData = (try? JSONEncoder().encode(rules)) ?? Data()
        let configJSON = String(data: jsonData, encoding: .utf8) ?? ""

        var error: NSError?
        MobileStartWithConfig(port, configJSON, &error)
        if let error = error {
            errorMessage = error.localizedDescription
            return
        }
        isRunning = true
        localIP = MobileGetLocalIP()
        connectURL = MobileGetConnectURL()
        errorMessage = nil
        startPollingStats()
    }

    func stopServer() {
        MobileStop()
        isRunning = false
        stopPollingStats()
    }

    private func startPollingStats() {
        statsTimer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { [weak self] _ in
            self?.refreshStats()
        }
    }

    private func stopPollingStats() {
        statsTimer?.invalidate()
        statsTimer = nil
    }

    func refreshStats() {
        let json = MobileGetStats()
        guard let data = json.data(using: .utf8) else { return }
        if let decoded = try? JSONDecoder().decode(GameStats.self, from: data) {
            DispatchQueue.main.async {
                self.stats = decoded
            }
        }
    }
}

struct GameStats: Codable {
    var version: String = ""
    var uptime: String = ""
    var currentPlayers: Int = 0
    var peakPlayers: Int = 0
    var aiCount: Int = 0
    var foodCount: Int = 0
    var totalKills: Int64 = 0
    var totalJoins: Int64 = 0
    var avgTickMs: Double = 0
    var bandwidthKBps: Double = 0
    var leaderboard: [LeaderboardEntry] = []
}

struct LeaderboardEntry: Codable, Identifiable {
    var name: String = ""
    var score: Int = 0
    var isAI: Bool = true
    var alive: Bool = true

    var id: String { "\(name)-\(score)" }
}
