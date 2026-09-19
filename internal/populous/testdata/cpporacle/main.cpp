// Fixtures around the mechanically extracted original function bodies.
int main(int argc, char **argv) {
    if (argc != 2) return 1;
    const std::string mode = argv[1];
    if (mode == "terrain") {
        int seed, before, setup;
        while (std::scanf("%d%d%d", &seed, &before, &setup) == 3) {
            CPopulous w;
            w.seed = static_cast<SHORT>(seed);
            for (int i = 0; i < before; i++) w.newrand();
            w.make_alt();
            w.make_map(0, 0, 63, 63);
            w.make_woods_rocks();
            // setup_display increments seed after people placement, which does
            // not consume RNG or change these landscape arrays in this fixture.
            if (setup) w.seed = static_cast<SHORT>(w.seed + 1);
            std::printf("%u %llu %llu %llu %llu\n",
                static_cast<unsigned>(static_cast<USHORT>(w.seed)),
                static_cast<unsigned long long>(hashaltitude(w.alt)),
                static_cast<unsigned long long>(hashbytes(w.map_alt, 4096)),
                static_cast<unsigned long long>(hashbytes(w.map_blk, 4096)),
                static_cast<unsigned long long>(hashbytes(w.map_bk2, 4096)));
        }
    } else if (mode == "battle") {
        int seed, player;
        while (std::scanf("%d%d", &seed, &player) == 2) {
            CPopulous w;
            w.seed = static_cast<SHORT>(seed);
            w.peeps[0].player = player;
            w.peeps[0].population = 1000;
            w.peeps[0].weapons = 3;
            w.peeps[0].battle_population = 1;
            w.peeps[1].player = player ^ 1;
            w.peeps[1].population = 2000;
            w.peeps[1].weapons = 5;
            w.do_battle(&w.peeps[0], 0);
            std::printf("%d %d %u\n", w.peeps[0].population,
                w.peeps[1].population,
                static_cast<unsigned>(static_cast<USHORT>(w.seed)));
        }
    } else if (mode == "join") {
        int player;
        while (std::scanf("%d", &player) == 1) {
            CPopulous w;
            w.peeps[0].player = player;
            w.peeps[0].population = 1000;
            w.peeps[0].weapons = 20;
            w.peeps[0].iq = 4;
            w.peeps[0].status = 1;
            w.peeps[0].head_for = &w.peeps[2];
            w.peeps[0].flags = 2;
            w.peeps[1].player = player;
            w.peeps[1].population = 100;
            w.peeps[1].weapons = 1;
            w.peeps[1].iq = 1;
            w.peeps[1].status = 2;
            w.peeps[1].flags = 1 | 8;
            w.peeps[1].frame = 71;
            w.peeps[1].battle_population = 2;
            w.peeps[2].population = 500;
            w.peeps[2].player = player ^ 1;
            w.peeps[2].flags = 8;
            w.peeps[2].battle_population = 1;
            w.magnet[player].carried = 1;
            w.magnet[player].population = 1000;
            w.join_battle(0, 2);
            std::printf("%d %d %d %d %d %d %d %d %d %d\n",
                w.peeps[0].population, w.peeps[1].population,
                w.peeps[1].head_for == &w.peeps[2], w.peeps[1].weapons,
                w.peeps[1].iq, w.peeps[1].status, w.peeps[1].flags,
                w.peeps[1].frame, static_cast<int>(w.magnet[player].population),
                w.magnet[player].carried);
        }
    } else {
        return 1;
    }
    return std::ferror(stdin) ? 1 : 0;
}
