add_library(proteacommon${QT_MAJOR_VERSION} OBJECT proteaboxshadowrenderer.cpp)

set_target_properties(proteacommon${QT_MAJOR_VERSION} PROPERTIES POSITION_INDEPENDENT_CODE TRUE)

target_link_libraries(proteacommon${QT_MAJOR_VERSION}
    PUBLIC
        Qt${QT_MAJOR_VERSION}::Core
        Qt${QT_MAJOR_VERSION}::Gui)
