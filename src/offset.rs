pub const DW_LOCAL_PLAYER_CONTROLLER: usize = 0x1813478;
pub const DW_ENTITY_LIST: usize = 0x1993438;
pub const DW_VIEW_MATRIX: usize = 0x1A21810;

// Netvars
pub const M_I_TEAM_NUM: usize = 0x3BF;
pub const M_I_HEALTH: usize = 0x334;
pub const M_H_PLAYER_PAWN: usize = 0x7EC;
pub const M_P_GAME_SCENE_NODE: usize = 0x318;
pub const M_V_ABS_ORIGIN: usize = 0x80;

#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub pawn_address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
