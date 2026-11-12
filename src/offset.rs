// Doğrudan client.dll tabanlı offsetler (Decimal değerler hex'e çevrildi)
pub const DW_LOCAL_PLAYER_CONTROLLER: usize = 0x2552CD8; // 39140664
pub const DW_ENTITY_LIST: usize = 0x2707250;           // 40931344
pub const DW_VIEW_MATRIX: usize = 0x2554600;           // 39163456

// Netvars
pub const M_I_TEAM_NUM: usize = 999;       // 0x3CF
pub const M_I_HEALTH: usize = 844;         // 0x34C
pub const M_H_PLAYER_PAWN: usize = 2348;   // 0x92C
pub const M_P_GAME_SCENE_NODE: usize = 816;// 0x330
pub const M_V_ABS_ORIGIN: usize = 5284;    // 0x14A4

#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub pawn_address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
