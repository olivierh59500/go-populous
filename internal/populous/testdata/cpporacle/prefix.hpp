// Test-only portable declarations. No original implementation is copied here.
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <string>

using SHORT = int16_t;
using USHORT = uint16_t;
using LONG = int32_t;
using UBYTE = uint8_t;
using BYTE = int8_t;

#define MAP_WIDTH 64
#define MAP_HEIGHT 64
#define END_WIDTH 65
#define END_HEIGHT 65
#define NO_WOODS 22
#define NO_TREES 30
#define TREE_BLOCK 50
#define ROCK_BLOCK 47
#define WATER_BLOCK 0
#define FLAT_BLOCK 15
#define ODSN(x) std::abort()

struct p_peeps {
    UBYTE flags = 0, player = 0, iq = 0, weapons = 0;
    SHORT population = 0;
    USHORT battle_population = 0;
    SHORT at_pos = 0, direction = 0, frame = 0;
    p_peeps *head_for = nullptr;
    SHORT in_out = 0;
    BYTE status = 0, magnet_last_move = 0;
};

struct m_magnet {
    SHORT carried = 0, go_to = 0, flags = 0, no_towns = 0;
    LONG population = 0, mana = 0;
};

class CPopulous {
public:
    SHORT seed = 0, toggle = 0, view_who = 0;
    SHORT xmin = 0, xmax = 0, ymin = 0, ymax = 0;
    int build_count = 0;
    // raise_point reads neighbouring vertices before checking bounds. Padding
    // gives those speculative reads defined storage; out-of-map calls return.
    SHORT altitude_storage[65 * 65 + 132]{};
    SHORT *alt = altitude_storage + 66;
    UBYTE map_blk[4096]{}, map_alt[4096]{}, map_bk2[4096]{};
    USHORT map_steps[4096]{};
    p_peeps peeps[212]{};
    m_magnet magnet[2]{};

    int newrand();
    SHORT raise_point(SHORT x, SHORT y);
    void make_alt();
    void make_thing(SHORT x, SHORT y);
    void make_map(SHORT x1, SHORT y1, SHORT x2, SHORT y2);
    void make_woods_rocks();
    void do_battle(p_peeps *peep, SHORT peep_pos);
    void join_battle(SHORT pos1, SHORT pos2);

    void a_putpixel(SHORT, SHORT) {}
    // Combat fixtures keep both sides alive: presentation does not affect the
    // population/RNG under test. Any unimplemented death/victory path must fail.
    int set_frame(p_peeps *) { return 0; }
    void zero_population(p_peeps *, SHORT) { std::abort(); }
    void battle_over(SHORT, SHORT) { std::abort(); }
};

static uint64_t hashbytes(const UBYTE *values, int length) {
    uint64_t hash = 14695981039346656037ULL;
    for (int i = 0; i < length; i++) {
        hash ^= values[i];
        hash *= 1099511628211ULL;
    }
    return hash;
}

static uint64_t hashaltitude(const SHORT *values) {
    UBYTE little_endian[65 * 65 * 2];
    for (int i = 0; i < 65 * 65; i++) {
        USHORT value = static_cast<USHORT>(values[i]);
        little_endian[i * 2] = static_cast<UBYTE>(value);
        little_endian[i * 2 + 1] = static_cast<UBYTE>(value >> 8);
    }
    return hashbytes(little_endian, sizeof(little_endian));
}
