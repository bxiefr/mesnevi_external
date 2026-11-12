pub const DW_LOCAL_PLAYER_PAWN: usize = 39140664; // dwLocalPlayerPawn
pub const DW_ENTITY_LIST: usize = 40931344;       // dwEntityList
pub const DW_VIEW_MATRIX: usize = 39163456;       // dwViewMatrix

pub const M_I_TEAM_NUM: usize = 999;       // m_iTeamNum
pub const M_I_HEALTH: usize = 844;         // m_iHealth
pub const M_H_PLAYER_PAWN: usize = 2348;   // m_hPlayerPawn
pub const M_P_GAME_SCENE_NODE: usize = 816;// m_pGameSceneNode
pub const M_V_ABS_ORIGIN: usize = 5284;    // m_vOldOrigin

#[allow(dead_code)]
#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub pawn_address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
