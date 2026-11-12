pub const DW_LOCAL_PLAYER_CONTROLLER: usize = 19022536; // Doğrudan güncel decimal offset (veya hex karşılığı)
pub const DW_ENTITY_LIST: usize = 24874912;
pub const DW_VIEW_MATRIX: usize = 24908992;

pub const M_I_TEAM_NUM: usize = 999;
pub const M_I_HEALTH: usize = 844;
pub const M_H_PLAYER_PAWN: usize = 2348;
pub const M_P_GAME_SCENE_NODE: usize = 816;
pub const M_V_ABS_ORIGIN: usize = 5284;

#[allow(dead_code)]
#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub pawn_address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
