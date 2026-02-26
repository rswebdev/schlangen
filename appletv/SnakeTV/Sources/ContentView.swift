import SwiftUI

struct ContentView: View {
    @EnvironmentObject var server: ServerManager

    var body: some View {
        if server.isRunning {
            DashboardView()
        } else {
            StartView()
        }
    }
}

// MARK: - Start Screen

struct StartView: View {
    @EnvironmentObject var server: ServerManager

    var body: some View {
        VStack(spacing: 40) {
            Text("Snake.io")
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

            Button(action: { server.startServer() }) {
                HStack(spacing: 12) {
                    Image(systemName: "play.fill")
                    Text("Start Server")
                }
                .font(.title2)
                .padding(.horizontal, 40)
                .padding(.vertical, 16)
            }
            .buttonStyle(.borderedProminent)
            .tint(.red)

            Text("Players connect via phone browser on the same WiFi")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }
}

// MARK: - Dashboard (Server Running)

struct DashboardView: View {
    @EnvironmentObject var server: ServerManager

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
                    .font(.system(.title3, design: .monospaced))
                    .foregroundColor(.blue)

                Text("Join on any phone browser")
                    .font(.caption)
                    .foregroundColor(.secondary)

                Spacer()

                Button(action: { server.stopServer() }) {
                    HStack {
                        Image(systemName: "stop.fill")
                        Text("Stop Server")
                    }
                    .font(.callout)
                }
                .buttonStyle(.bordered)
                .tint(.red)
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
                    Text("Snake.io Server")
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

                // Leaderboard
                Text("LEADERBOARD")
                    .font(.caption)
                    .fontWeight(.semibold)
                    .foregroundColor(.secondary)
                    .tracking(1)

                ScrollView {
                    VStack(spacing: 2) {
                        ForEach(Array(server.stats.leaderboard.prefix(10).enumerated()), id: \.offset) { index, entry in
                            LeaderboardRow(rank: index + 1, entry: entry)
                        }
                    }
                }
            }
            .padding()
        }
        .padding()
    }
}

// MARK: - Components

struct StatCard: View {
    let label: String
    let value: String
    let color: Color

    var body: some View {
        VStack(spacing: 4) {
            Text(label)
                .font(.caption2)
                .foregroundColor(.secondary)
                .textCase(.uppercase)
            Text(value)
                .font(.system(size: 36, weight: .bold, design: .rounded))
                .foregroundColor(color)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .background(Color.gray.opacity(0.15))
        .cornerRadius(12)
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
                .frame(width: 30)

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
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(rank <= 3 ? Color.yellow.opacity(0.05) : Color.clear)
        .cornerRadius(8)
    }
}
