// Güncel client.dll base offsetleri (Hex formatında)
pub const DW_LOCAL_PLAYER_CONTROLLER: usize = 0x2552CD8; // 39140664
pub const DW_ENTITY_LIST: usize = 0x2707250;           // 40931344
pub const DW_VIEW_MATRIX: usize = 0x2554600;           // 39163456

// Güncel Netvar Offsetleri (Decimal / Ham Byte Değerleri)
pub const M_I_TEAM_NUM: usize = 999;       // m_iTeamNum
pub const M_I_HEALTH: usize = 844;         // m_iHealth
pub const M_H_PLAYER_PAWN: usize = 2348;   // m_hPlayerPawn
pub const M_P_GAME_SCENE_NODE: usize = 816;// m_pGameSceneNode
pub const M_V_ABS_ORIGIN: usize = 5284;    // m_vOldOrigin (m_vOldOrigin)

#[derive(Clone, Copy)]
pub struct Player {
    pub address: usize,
    pub pawn_address: usize,
    pub team: i32,
    pub health: i32,
    pub pos: [f32; 3],
}
