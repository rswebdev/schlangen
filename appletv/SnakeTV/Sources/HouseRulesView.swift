import SwiftUI

// MARK: - House Rules Model

struct HouseRules: Codable {
    var worldSize: Int = 10000
    var foodCount: Int = 3000
    var aiCount: Int = 30
    var baseSpeed: Double = 3.2
    var boostSpeed: Double = 5.5

    static let defaults = HouseRules()
}

// MARK: - Preset Types

enum WorldSizePreset: String, CaseIterable {
    case small = "Small"
    case medium = "Medium"
    case large = "Large"

    var value: Int {
        switch self {
        case .small: return 5000
        case .medium: return 10000
        case .large: return 15000
        }
    }

    static func from(_ v: Int) -> WorldSizePreset {
        switch v {
        case ...7500: return .small
        case 7501...12500: return .medium
        default: return .large
        }
    }
}

enum SpeedPreset: String, CaseIterable {
    case slow = "Slow"
    case normal = "Normal"
    case fast = "Fast"
    case insane = "Insane"

    var value: Double {
        switch self {
        case .slow: return 2.0
        case .normal: return 3.2
        case .fast: return 4.5
        case .insane: return 6.0
        }
    }

    static func from(_ v: Double) -> SpeedPreset {
        switch v {
        case ...2.5: return .slow
        case 2.6...3.8: return .normal
        case 3.9...5.2: return .fast
        default: return .insane
        }
    }
}

enum BoostPreset: String, CaseIterable {
    case normal = "Normal"
    case fast = "Fast"
    case turbo = "Turbo"

    var value: Double {
        switch self {
        case .normal: return 5.5
        case .fast: return 7.0
        case .turbo: return 9.0
        }
    }

    static func from(_ v: Double) -> BoostPreset {
        switch v {
        case ...6.2: return .normal
        case 6.3...8.0: return .fast
        default: return .turbo
        }
    }
}

enum FoodPreset: String, CaseIterable {
    case sparse = "Sparse"
    case normal = "Normal"
    case dense = "Dense"

    var value: Int {
        switch self {
        case .sparse: return 1000
        case .normal: return 3000
        case .dense: return 5000
        }
    }

    static func from(_ v: Int) -> FoodPreset {
        switch v {
        case ...2000: return .sparse
        case 2001...4000: return .normal
        default: return .dense
        }
    }
}

// MARK: - House Rules View

struct HouseRulesView: View {
    @EnvironmentObject var server: ServerManager
    @State private var rules = HouseRules.defaults

    var body: some View {
        VStack(spacing: 30) {
            Text("House Rules")
                .font(.system(size: 48, weight: .bold))

            // Settings grid
            VStack(spacing: 16) {
                PresetRow(
                    label: "World Size",
                    icon: "map",
                    options: WorldSizePreset.allCases.map { $0.rawValue },
                    selected: WorldSizePreset.from(rules.worldSize).rawValue,
                    onSelect: { name in
                        if let p = WorldSizePreset.allCases.first(where: { $0.rawValue == name }) {
                            rules.worldSize = p.value
                        }
                    }
                )

                PresetRow(
                    label: "Game Speed",
                    icon: "hare",
                    options: SpeedPreset.allCases.map { $0.rawValue },
                    selected: SpeedPreset.from(rules.baseSpeed).rawValue,
                    onSelect: { name in
                        if let p = SpeedPreset.allCases.first(where: { $0.rawValue == name }) {
                            rules.baseSpeed = p.value
                        }
                    }
                )

                PresetRow(
                    label: "Boost Speed",
                    icon: "bolt.fill",
                    options: BoostPreset.allCases.map { $0.rawValue },
                    selected: BoostPreset.from(rules.boostSpeed).rawValue,
                    onSelect: { name in
                        if let p = BoostPreset.allCases.first(where: { $0.rawValue == name }) {
                            rules.boostSpeed = p.value
                        }
                    }
                )

                PresetRow(
                    label: "Food Density",
                    icon: "circle.circle.fill",
                    options: FoodPreset.allCases.map { $0.rawValue },
                    selected: FoodPreset.from(rules.foodCount).rawValue,
                    onSelect: { name in
                        if let p = FoodPreset.allCases.first(where: { $0.rawValue == name }) {
                            rules.foodCount = p.value
                        }
                    }
                )

                StepperRow(
                    label: "AI Snakes",
                    icon: "cpu",
                    value: $rules.aiCount,
                    range: 0...50,
                    step: 5
                )
            }
            .padding(.horizontal, 60)

            Spacer()

            // Bottom buttons
            HStack(spacing: 40) {
                Button(action: { rules = HouseRules.defaults }) {
                    HStack(spacing: 8) {
                        Image(systemName: "arrow.counterclockwise")
                        Text("Reset Defaults")
                    }
                    .font(.callout)
                }
                .buttonStyle(.bordered)

                Button(action: { server.startServer(with: rules) }) {
                    HStack(spacing: 12) {
                        Image(systemName: "play.fill")
                        Text("Start Game")
                    }
                    .font(.title3)
                    .padding(.horizontal, 30)
                    .padding(.vertical, 10)
                }
                .buttonStyle(.borderedProminent)
                .tint(.red)
            }

            if let err = server.errorMessage {
                Text(err)
                    .foregroundColor(.red)
                    .font(.caption)
            }
        }
        .padding(.vertical, 40)
    }
}

// MARK: - Row Components

struct PresetRow: View {
    let label: String
    let icon: String
    let options: [String]
    let selected: String
    let onSelect: (String) -> Void

    var body: some View {
        HStack {
            Label(label, systemImage: icon)
                .font(.title3)
                .frame(width: 220, alignment: .leading)

            Spacer()

            HStack(spacing: 12) {
                ForEach(options, id: \.self) { option in
                    Button(action: { onSelect(option) }) {
                        Text(option)
                            .font(.callout)
                            .fontWeight(option == selected ? .bold : .regular)
                            .padding(.horizontal, 20)
                            .padding(.vertical, 8)
                    }
                    .buttonStyle(.bordered)
                    .tint(option == selected ? .red : .gray)
                }
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 8)
    }
}

struct StepperRow: View {
    let label: String
    let icon: String
    @Binding var value: Int
    let range: ClosedRange<Int>
    let step: Int

    var body: some View {
        HStack {
            Label(label, systemImage: icon)
                .font(.title3)
                .frame(width: 220, alignment: .leading)

            Spacer()

            HStack(spacing: 20) {
                Button(action: {
                    if value - step >= range.lowerBound { value -= step }
                }) {
                    Image(systemName: "minus.circle.fill")
                        .font(.title2)
                }
                .buttonStyle(.plain)
                .disabled(value <= range.lowerBound)

                Text("\(value)")
                    .font(.system(size: 28, weight: .bold, design: .rounded))
                    .frame(width: 60)

                Button(action: {
                    if value + step <= range.upperBound { value += step }
                }) {
                    Image(systemName: "plus.circle.fill")
                        .font(.title2)
                }
                .buttonStyle(.plain)
                .disabled(value >= range.upperBound)
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 8)
    }
}
