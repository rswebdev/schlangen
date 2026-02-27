import SwiftUI

// MARK: - House Rules Model

struct HouseRules: Codable {
    var worldSize: Int = 5000
    var foodCount: Int = 1500
    var aiCount: Int = 10
    var baseSpeed: Double = 3.2
    var boostSpeed: Double = 5.5
    var tickRate: Int = 30

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

enum AICountPreset: String, CaseIterable {
    case none = "None"
    case few = "Few"
    case some = "Some"
    case many = "Many"
    case swarm = "Swarm"

    var value: Int {
        switch self {
        case .none: return 0
        case .few: return 10
        case .some: return 20
        case .many: return 30
        case .swarm: return 50
        }
    }

    static func from(_ v: Int) -> AICountPreset {
        switch v {
        case ...4: return .none
        case 5...14: return .few
        case 15...24: return .some
        case 25...39: return .many
        default: return .swarm
        }
    }
}

// MARK: - House Rules View

struct HouseRulesView: View {
    @EnvironmentObject var server: ServerManager
    @State private var rules = HouseRules.defaults

    var body: some View {
        VStack(spacing: 24) {
            Text("House Rules")
                .font(.title)
                .fontWeight(.bold)

            VStack(spacing: 12) {
                PickerRow(
                    label: "World Size",
                    icon: "map",
                    selection: WorldSizePreset.from(rules.worldSize),
                    onSelect: { rules.worldSize = $0.value }
                )

                PickerRow(
                    label: "Game Speed",
                    icon: "hare",
                    selection: SpeedPreset.from(rules.baseSpeed),
                    onSelect: { rules.baseSpeed = $0.value }
                )

                PickerRow(
                    label: "Boost Speed",
                    icon: "bolt.fill",
                    selection: BoostPreset.from(rules.boostSpeed),
                    onSelect: { rules.boostSpeed = $0.value }
                )

                PickerRow(
                    label: "Food Density",
                    icon: "circle.circle.fill",
                    selection: FoodPreset.from(rules.foodCount),
                    onSelect: { rules.foodCount = $0.value }
                )

                PickerRow(
                    label: "AI Snakes",
                    icon: "cpu",
                    selection: AICountPreset.from(rules.aiCount),
                    onSelect: { rules.aiCount = $0.value }
                )
            }
            .padding(.horizontal, 60)

            Spacer()

            HStack(spacing: 40) {
                Button(action: { rules = HouseRules.defaults }) {
                    HStack(spacing: 8) {
                        Image(systemName: "arrow.counterclockwise")
                        Text("Reset Defaults")
                    }
                }
                .buttonStyle(.bordered)

                Button(action: { server.startServer(with: rules) }) {
                    HStack(spacing: 12) {
                        Image(systemName: "play.fill")
                        Text("Start Game")
                    }
                    .padding(.horizontal, 20)
                    .padding(.vertical, 6)
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

struct PickerRow<T: RawRepresentable & CaseIterable & Hashable>: View where T.RawValue == String, T.AllCases: RandomAccessCollection {
    let label: String
    let icon: String
    let selection: T
    let onSelect: (T) -> Void

    @State private var current: T

    init(label: String, icon: String, selection: T, onSelect: @escaping (T) -> Void) {
        self.label = label
        self.icon = icon
        self.selection = selection
        self.onSelect = onSelect
        self._current = State(initialValue: selection)
    }

    var body: some View {
        HStack {
            Label(label, systemImage: icon)
                .frame(width: 500, alignment: .leading)

            Spacer()

            Picker(label, selection: $current) {
                ForEach(Array(T.allCases), id: \.self) { option in
                    Text(option.rawValue).tag(option)
                }
            }
            .onChange(of: current) { newValue in
                onSelect(newValue)
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 4)
    }
}
