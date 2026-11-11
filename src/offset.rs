pub const OFFSET_CLIENT_STATE: usize = 0x0; // Gerekirse güncellenir
pub const DW_LOCAL_PLAYER_CONTROLLER: usize = 0x1813478; // Örnek güncel offset (projedeki offsets.json ile eşleyebilirsin)
pub const DW_ENTITY_LIST: usize = 0x1993438;
pub const DW_VIEW_MATRIX: usize = 0x1A21810;

// Oyuncu Yapısı (Player Struct)
#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
