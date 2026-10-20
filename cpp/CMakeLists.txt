cmake_minimum_required(VERSION 3.16)
project(protea)
set(PROJECT_VERSION "0.1.0")

set(KF5_MIN_VERSION "5.102.0")
set(KF6_MIN_VERSION "6.30.0")
set(KDE_COMPILERSETTINGS_LEVEL "5.82")
set(QT5_MIN_VERSION "5.15.2")
set(QT_MIN_VERSION "6.11.0")

include(GenerateExportHeader)
include(FeatureSummary)

find_package(ECM ${KF5_MIN_VERSION} REQUIRED NO_MODULE)
set(CMAKE_MODULE_PATH ${ECM_MODULE_PATH})

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# We need this because we can't include KDECMakeSettings here, because that
# would need KDEInstallDirs, which we can only include in the qt{5,6} builds
# to get the correct installation dirs for each Qt version.
option(BUILD_TESTING "Build the testing tree." ON)
if(BUILD_TESTING)
    enable_testing()
endif()

include(ECMInstallIcons)
include(ECMQtDeclareLoggingCategory)
include(KDECompilerSettings NO_POLICY_SCOPE)
include(KDEClangFormat)


option(BUILD_QT5 "Build Qt5 style" OFF)
option(BUILD_QT6 "Build with Qt6" ON)
option(BUILD_WITH_QTQUICK "Build with Qt Quick features enabled" ON)

set(QT_NO_CREATE_VERSIONLESS_TARGETS ON)
set(QT_NO_CREATE_VERSIONLESS_FUNCTIONS ON)

function(build_Qt5)
    set(QT_MAJOR_VERSION 5)
    include(KDEInstallDirs5)
    include(KDECMakeSettings)

    find_package(Qt5 ${QT5_MIN_VERSION} REQUIRED CONFIG COMPONENTS Widgets)

    if(UNIX AND NOT APPLE AND NOT ANDROID)
        find_package(Qt5 ${QT5_MIN_VERSION} CONFIG REQUIRED DBus)
        set(HAVE_QTDBUS ${Qt5DBus_FOUND})

        find_package(KF5FrameworkIntegration ${KF5_MIN_VERSION} CONFIG )
        set_package_properties(KF${QT_MAJOR_VERSION}FrameworkIntegration PROPERTIES
            DESCRIPTION "KF${QT_MAJOR_VERSION} Framework Integration"
            URL "https://projects.kde.org/projects/frameworks/frameworkintegration"
            TYPE OPTIONAL
            PURPOSE "Required to use KStyle convenience functionalities in style")

        find_package(Qt5 ${QT5_MIN_VERSION} OPTIONAL_COMPONENTS X11Extras)
        set_package_properties(KF${QT_MAJOR_VERSION}FrameworkIntegration PROPERTIES
            DESCRIPTION "KF${QT_MAJOR_VERSION} Framework Integration"
            URL "https://projects.kde.org/projects/frameworks/frameworkintegration"
            TYPE OPTIONAL
            PURPOSE "Required to use KStyle convenience functionalities in style")
    endif()

    find_package(KF5 ${KF5_MIN_VERSION} REQUIRED COMPONENTS
        CoreAddons
        Config
        ConfigWidgets
        GuiAddons
        IconThemes
        WindowSystem)

    if(BUILD_WITH_QTQUICK)
        find_package(Qt5 ${QT5_MIN_VERSION} REQUIRED COMPONENTS Quick)
    endif()

    set(PROTEA_HAVE_KSTYLE ${KF5FrameworkIntegration_FOUND})
    set(PROTEA_HAVE_QTQUICK ${Qt5Quick_FOUND})

    add_subdirectory(kstyle kstyle5)
    add_subdirectory(libproteacommon libproteacommon5)

    unset(QUERY_EXECUTABLE CACHE)
endfunction()
if(BUILD_QT5)
    build_Qt5()
endif()

function(build_Qt6)
    set(QT_MAJOR_VERSION 6)
    include(KDEInstallDirs6)
    include(KDECMakeSettings)

    if(UNIX AND NOT APPLE AND NOT ANDROID)
        find_package(Qt6 ${REQUIRED_QT_VERSION} CONFIG REQUIRED DBus)
        set(HAVE_QTDBUS ${Qt6DBus_FOUND})

        option(WITH_DECORATIONS "Build Protea window decorations for KWin" ON)

        find_package(KF6FrameworkIntegration ${KF6_MIN_VERSION} CONFIG )
        set_package_properties(KF${QT_MAJOR_VERSION}FrameworkIntegration PROPERTIES
            DESCRIPTION "KF6 Framework Integration"
            URL "https://projects.kde.org/projects/frameworks/frameworkintegration"
            TYPE OPTIONAL
            PURPOSE "Required to use KStyle convenience functionalities in style")

        find_package(KF6KCMUtils ${KF6_MIN_VERSION})
        set_package_properties(KF6KCMUtils PROPERTIES
            TYPE REQUIRED
            DESCRIPTION "Helps create configuration modules"
            PURPOSE "KCMUtils used for the configuration modules or the decoration and Qt Style"
        )
    else()
        set(WITH_DECORATIONS OFF)
    endif()

    find_package(Qt6 ${QT_MIN_VERSION} REQUIRED CONFIG COMPONENTS Widgets Core Svg)

    find_package(KF6 ${KF6_MIN_VERSION} REQUIRED COMPONENTS
        CoreAddons
        ColorScheme
        Config
        GuiAddons
        I18n
        IconThemes
        WindowSystem)

    if(BUILD_WITH_QTQUICK)
        find_package(Qt6 ${QT_MIN_VERSION} REQUIRED COMPONENTS Quick)
    endif()

    set(PROTEA_HAVE_KSTYLE ${KF6FrameworkIntegration_FOUND})
    set(PROTEA_HAVE_QTQUICK ${Qt6Quick_FOUND})

    add_subdirectory(kstyle kstyle6)
    add_subdirectory(libproteacommon libproteacommon6)

    if(WITH_DECORATIONS)
        find_package(KDecoration3 REQUIRED)
        add_subdirectory(kdecoration)
    endif()

    ki18n_install(po)

    unset(QUERY_EXECUTABLE CACHE)
endfunction()
if(BUILD_QT6)
    build_Qt6()
endif()

# add clang-format target for all our real source files
file(GLOB_RECURSE ALL_CLANG_FORMAT_SOURCE_FILES *.cpp *.h)
kde_clang_format(${ALL_CLANG_FORMAT_SOURCE_FILES})

feature_summary(WHAT ALL FATAL_ON_MISSING_REQUIRED_PACKAGES)
