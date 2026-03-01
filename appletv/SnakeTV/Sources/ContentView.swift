import SwiftUI

struct ContentView: View {
    @EnvironmentObject var server: ServerManager
    @State private var showHouseRules = false

    var body: some View {
        if server.isRunning {
            DashboardView()
        } else if showHouseRules {
            HouseRulesView()
        } else {
            StartView(showHouseRules: $showHouseRules)
        }
    }
}

// MARK: - Start Screen

struct StartView: View {
    @EnvironmentObject var server: ServerManager
    @Binding var showHouseRules: Bool

    var body: some View {
        VStack(spacing: 40) {
            Text("Schlangen.TV")
                .font(.system(size: 72, weight: .bold))
                .foregroundStyle(
                    LinearGradient(
                        colors: [.red, .pink],
                        startPoint: .leading,
                        endPoint: .trailing
                    )
                )

            Text("Party Mode")
                .font(.system(size: 28))
                .foregroundColor(.secondary)

            if let err = server.errorMessage {
                Text(err)
                    .foregroundColor(.red)
                    .font(.caption)
            }

            Button(action: { showHouseRules = true }) {
                HStack(spacing: 12) {
                    Image(systemName: "play.fill")
                    Text("Start Game")
                }
                .font(.title2)
                .padding(.horizontal, 40)
                .padding(.vertical, 16)
            }
            .buttonStyle(.borderedProminent)

            Text("Players connect via phone browser on the same WiFi")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }
}

// MARK: - Dashboard (Server Running)

struct DashboardView: View {
    @EnvironmentObject var server: ServerManager
    @State private var showSpectator = false
    @State private var spectatingName = ""

    var body: some View {
        HStack(spacing: 40) {
            // Left side: QR code + connection info
            VStack(spacing: 24) {
                Text("Scan to Play")
                    .font(.title2)
                    .fontWeight(.semibold)

                QRCodeView(url: server.connectURL)
                    .frame(width: 280, height: 280)
                    .background(Color.white)
                    .cornerRadius(16)

                Text(server.connectURL)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundColor(.blue)

                if let rules = server.activeRules {
                    VStack() {
                        Text("Rules")
                            .font(.caption2)
                            .fontWeight(.semibold)
                            .foregroundColor(.secondary)
                            .tracking(1)
                        VStack() {
                            RuleChip(icon: "map", label: WorldSizePreset.from(rules.worldSize).rawValue)
                            RuleChip(icon: "hare", label: SpeedPreset.from(rules.baseSpeed).rawValue)
                            RuleChip(icon: "bolt.fill", label: BoostPreset.from(rules.boostSpeed).rawValue)
                            RuleChip(icon: "circle.circle.fill", label: FoodPreset.from(rules.foodCount).rawValue)
                            RuleChip(icon: "cpu", label: AICountPreset.from(rules.aiCount).rawValue)
                        }
                    }
                    .padding(0)
                }

                Spacer()

                Button(action: { server.stopServer() }) {
                    HStack {
                        Image(systemName: "stop.fill")
                        Text("Stop Server")
                    }
                    .font(.callout)
                }
                .buttonStyle(.bordered)
            }
            .frame(width: 360)
            .padding()

            // Right side: Live stats + leaderboard
            VStack(alignment: .leading, spacing: 20) {
                // Header
                HStack {
                    Circle()
                        .fill(.green)
                        .frame(width: 12, height: 12)
                    Text("Schlangen.TV Server")
                        .font(.title2)
                        .fontWeight(.bold)
                    Text("v\(server.stats.version)")
                        .font(.callout)
                        .foregroundColor(.secondary)
                    Spacer()
                    Text(server.stats.uptime)
                        .font(.callout)
                        .foregroundColor(.secondary)
                }

                // Stats cards
                LazyVGrid(columns: [
                    GridItem(.flexible()),
                    GridItem(.flexible()),
                    GridItem(.flexible()),
                    GridItem(.flexible())
                ], spacing: 12) {
                    StatCard(label: "Players", value: "\(server.stats.currentPlayers)", color: .blue)
                    StatCard(label: "Peak", value: "\(server.stats.peakPlayers)", color: .purple)
                    StatCard(label: "AI Snakes", value: "\(server.stats.aiCount)", color: .orange)
                    StatCard(label: "Total Kills", value: "\(server.stats.totalKills)", color: .red)
                }

                // Runtime / performance stats
                LazyVGrid(columns: [
                    GridItem(.flexible()),
                    GridItem(.flexible()),
                    GridItem(.flexible()),
                    GridItem(.flexible())
                ], spacing: 12) {
                    StatCard(
                        label: "Memory",
                        value: String(format: "%.1f / %.0f MB", server.stats.memAllocMB, server.stats.memSysMB),
                        color: .cyan
                    )
                    StatCard(
                        label: "Goroutines",
                        value: "\(server.stats.numGoroutines)",
                        color: .mint
                    )
                    StatCard(
                        label: "GC Pause",
                        value: String(format: "%.2f ms", server.stats.gcPauseMs),

                        color: .teal
                    )
                    StatCard(
                        label: "Tick Load",
                        value: String(format: "%.0f%%", min(server.stats.avgTickMs / 16.67 * 100, 999)),
                        color: tickLoadColor(server.stats.avgTickMs)
                    )
                }

                // Leaderboard
                Text("LEADERBOARD")
                    .font(.caption)
                    .fontWeight(.semibold)
                    .foregroundColor(.secondary)
                    .tracking(1)

                ScrollView {
                    VStack(spacing: 2) {
                        ForEach(Array(server.stats.leaderboard.prefix(10).enumerated()), id: \.offset) { index, entry in
                            Button(action: {
                                spectatingName = entry.name
                                showSpectator = true
                            }) {
                                LeaderboardRow(rank: index + 1, entry: entry)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
            }
            .padding()
        }
        .padding()
        .onAppear { UIApplication.shared.isIdleTimerDisabled = true }
        .onDisappear { UIApplication.shared.isIdleTimerDisabled = false }
        .fullScreenCover(isPresented: $showSpectator) {
            SpectatorView(port: server.port, spectateName: spectatingName)
                .ignoresSafeArea()
        }
    }
}

// MARK: - Components

struct StatCard: View {
    let label: String
    let value: String
    var subtitle: String? = nil
    let color: Color

    var body: some View {
        VStack(spacing: 4) {
            Text(label)
                .font(.caption2)
                .foregroundColor(.secondary)
                .textCase(.uppercase)
            Text(value)
                .font(.system(size: 30, weight: .bold, design: .rounded))
                .foregroundColor(color)
            if let subtitle = subtitle {
                Text(subtitle)
                    .font(.caption2)
                    .foregroundColor(.secondary)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .background(Color.gray.opacity(0.15))
        .cornerRadius(12)
    }
}

private func tickLoadColor(_ avgTickMs: Double) -> Color {
    let pct = avgTickMs / 16.67 * 100
    if pct < 50 { return .green }
    if pct < 80 { return .yellow }
    return .red
}

struct RuleChip: View {
    let icon: String
    let label: String

    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: icon)
                .font(.caption2)
                .foregroundColor(.secondary)
            Text(label)
                .font(.caption)
                .foregroundColor(.primary)
        }
    }
}

struct LeaderboardRow: View {
    let rank: Int
    let entry: LeaderboardEntry

    var body: some View {
        HStack {
            Text("\(rank)")
                .font(.callout)
                .fontWeight(.bold)
                .foregroundColor(.secondary)
                .frame(width: 40)

            Text(entry.name)
                .font(.callout)

            Spacer()

            Text("\(entry.score)")
                .font(.callout)
                .fontWeight(.semibold)

            Text(entry.isAI ? "AI" : "Player")
                .font(.caption2)
                .fontWeight(.semibold)
                .padding(.horizontal, 8)
                .padding(.vertical, 3)
                .background(entry.isAI ? Color.purple.opacity(0.3) : Color.blue.opacity(0.3))
                .cornerRadius(4)
                .frame(minWidth: 100)

        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(rank <= 3 ? Color.yellow.opacity(0.05) : Color.clear)
        .cornerRadius(8)
    }
}

#Preview {
    ContentView()
        .environmentObject({
            let s = ServerManager()
            s.isRunning = true
            s.stats.currentPlayers = 100
            s.stats.peakPlayers = 150
            s.stats.aiCount = 20
            s.stats.gcPauseMs = 0.03
            s.stats.avgTickMs = 0.3
            s.stats.totalKills = 1000000
            s.stats.memSysMB = 120
            s.stats.memAllocMB = 100
            s.stats.uptime = "1h 23m 4s"
            s.stats.version = "1.0.0"
            s.activeRules = HouseRules()
            s.connectURL = "http://192.168.1.42:8080"

            s.stats.leaderboard = (0..<10).map { index in
                let rank = index + 1
                return LeaderboardEntry(name: "Player \(rank)", score: 1000 - rank * 80, isAI: rank.isMultiple(of: 2))
            }
            
            return s
        }())
}
