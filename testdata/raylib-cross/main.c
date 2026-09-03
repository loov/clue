#include <raylib.h>

int main(void) {
    InitWindow(800, 450, "Clue + raylib");

    while (!WindowShouldClose()) {
        BeginDrawing();
        ClearBackground(RAYWHITE);
        DrawText("Built with Clue", 300, 220, 20, DARKGRAY);
        EndDrawing();
    }

    CloseWindow();
    return 0;
}
